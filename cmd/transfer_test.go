/*
 *
 * Copyright © 2020 nicksherron <nsherron90@gmail.com>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package cmd

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/icrowley/fake"
	"github.com/magiconair/properties/assert"
	"github.com/nicksherron/bashhub-server/internal"
)

var (
	testWork         bool
	testDir          string
	srcCmd           *exec.Cmd
	dstCmd           *exec.Cmd
	sessionStartTime int64
	commandsN        int
	srcPostgres      string
	dstPostgres      string
	dst              user
	src              user
)

type user struct {
	url       string
	username  string
	pass      string
	db        string
	httpLog   string
	stderrLog io.Writer
}

func init() {
	flag.StringVar(&srcURL, "src-url", "http://localhost:55555", "source url ")
	flag.StringVar(&srcUser, "src-user", "tester", "source username")
	flag.StringVar(&srcPass, "src-pass", "tester", "source password")
	flag.StringVar(&dstURL, "dst-url", "http://localhost:55556", "destination url")
	flag.StringVar(&dstUser, "dst-user", "tester", "destination username")
	flag.StringVar(&dstPass, "dst-pass", "tester", "destination password")
	flag.IntVar(&workers, "workers", 10, "max number of concurrent requests")
	flag.IntVar(&commandsN, "number", 200, "number of commmands to use for test")
	flag.BoolVar(&testWork, "testwork", false, "don't remove sqlite db and server log when done and print location")
	flag.StringVar(&srcPostgres, "src-postgres-uri", "", "postgres uri to use for postgres tests")
	flag.StringVar(&dstPostgres, "dst-postgres-uri", "", "postgres uri to use for postgres tests")

}

func (u user) startServer() (p *exec.Cmd, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		check(err)
	}
	parent := filepath.Dir(cwd)
	cmd := "go"
	args := []string{"run", ".", "-a", u.url, "--db", u.db, "--log", u.httpLog}
	if cmd, err = exec.LookPath(cmd); err == nil {
		var procAttr os.ProcAttr
		procAttr.Dir = parent
		procAttr.Files = []*os.File{os.Stdin,
			os.Stdout, os.Stderr}
		p := exec.Command(cmd, args...)
		p.Dir = parent
		p.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		p.Stderr = u.stderrLog
		return p, nil
	}
	return nil, err
}

func setup(srcDB string, dstDB string) {
	srcErr := filepath.Join(testDir, "src-stderr.log")
	srcStderrLog, err := os.Create(srcErr)
	check(err)

	src = user{
		url:       srcURL,
		username:  srcUser,
		pass:      srcPass,
		db:        srcDB,
		httpLog:   filepath.Join(testDir, "src-server.log"),
		stderrLog: srcStderrLog,
	}

	dstErr := filepath.Join(testDir, "dst-stderr.log")
	dstStderrLog, err := os.Create(dstErr)
	check(err)

	dst = user{
		url:       dstURL,
		username:  dstUser,
		pass:      dstPass,
		db:        dstDB,
		httpLog:   filepath.Join(testDir, "dst-server.log"),
		stderrLog: dstStderrLog,
	}

	srcCmd, err = src.startServer()
	check(err)
	err = srcCmd.Start()
	check(err)

	dstCmd, err = dst.startServer()
	check(err)
	err = dstCmd.Start()
	check(err)
	tries := 0

	for {
		if src.ping() == nil && dst.ping() == nil {
			break
		}
		tries++
		if tries > 10 {
			log.Fatal("failed connecting to servers after 10 attempts")
		}
		time.Sleep(2 * time.Second)
	}

	src.createUser()
	dst.createUser()
}

func TestMain(m *testing.M) {
	flag.Parse()
	var err error
	testDir, err = ioutil.TempDir("", "bashhub-server-test-")
	check(err)
	if testWork {
		log.Println("TESTWORK=", testDir)
	}
	defer cleanup()
	setup(filepath.Join(testDir, "src.db"), filepath.Join(testDir, "dst.db"))
	m.Run()

	if srcPostgres != "" && dstPostgres != "" {
		log.SetOutput(os.Stderr)
		log.Print("postgres tests")
		cleanup()
		testDir, err = ioutil.TempDir("", "bashhub-server-test-")
		check(err)
		if testWork {
			log.Println("TESTWORK=", testDir)
		}
		setup(srcPostgres, dstPostgres)
		m.Run()
	}
}

func (u user) ping() error {
	_, err := http.Get(fmt.Sprintf("%v/ping", u.url))
	if err != nil {
		return err
	}
	return nil
}

func (u user) createUser() {
	auth := map[string]interface{}{
		"email":    "foo@gmail.com",
		"Username": u.username,
		"password": u.pass,
	}

	payloadBytes, err := json.Marshal(auth)
	if err != nil {
		log.Fatal(err)
	}
	body := bytes.NewReader(payloadBytes)
	uri := fmt.Sprintf("%v/api/v1/user", u.url)
	req, err := http.NewRequest("POST", uri, body)

	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	defer resp.Body.Close()

}

func TestCreateToken(t *testing.T) {
	sysRegistered = false
	srcToken = getToken(srcURL, srcUser, srcPass)
	if srcToken == "" {
		t.Fatal("srcToken token is blank")
	}

	sysRegistered = false
	dstToken = getToken(dstURL, dstUser, dstPass)
	if dstToken == "" {
		t.Fatal("dstToken token is blank")
	}
}

func commandInsert() {
	counter := 0
	sessionStartTime = time.Now().Unix() * 1000
	var w sync.WaitGroup
	for i := 0; i < commandsN; i++ {
		w.Add(1)
		counter++
		go func() {
			defer w.Done()
			var tc internal.Command
			uid, err := uuid.NewRandom()
			check(err)

			tc.Command = fake.Words()
			tc.Path = "/dev/null"
			tc.Created = time.Now().Unix() * 1000
			tc.Uuid = uid.String()
			tc.ExitStatus = 0
			tc.SystemName = "system"
			tc.ProcessId = 1000
			tc.User.Username = srcUser
			tc.ProcessStartTime = sessionStartTime

			payloadBytes, err := json.Marshal(&tc)
			check(err)
			body := bytes.NewReader(payloadBytes)

			req, err := http.NewRequest("POST", fmt.Sprintf("%v/api/v1/command", srcURL), body)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Add("Authorization", srcToken)

			resp, err := http.DefaultClient.Do(req)
			defer resp.Body.Close()
		}()
		if counter > workers {
			w.Wait()
			counter = 0
		}

	}
	w.Wait()
}

func TestCommandList(t *testing.T) {
	commandInsert()
	cmdList = getCommandList()
	if len(cmdList) == 0 {
		t.Fatal("command list is empty")
	}
}

func TestTransfer(t *testing.T) {
	progress = true
	unique = false
	run()
	srcStatus := getStatus(t, srcURL, srcToken)
	dstStatus := getStatus(t, dstURL, dstToken)
	assert.Equal(t, srcStatus.TotalCommands, commandsN)
	assert.Equal(t, dstStatus.TotalCommands, srcStatus.TotalCommands)
}

func getStatus(t *testing.T, u string, token string) internal.Status {
	u = fmt.Sprintf("%v/api/v1/client-view/status?processId=1000&startTime=%v", u, sessionStartTime)
	req, err := http.NewRequest("GET", u, nil)
	check(err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var status internal.Status
	err = json.Unmarshal(body, &status)
	if err != nil {
		t.Fatal(err)
	}

	return status
}

func cleanup() {
	defer func() {
		if err := syscall.Kill(-srcCmd.Process.Pid, syscall.SIGKILL); err != nil {
			log.Println("failed to kill: ", err)
		}
		if err := syscall.Kill(-dstCmd.Process.Pid, syscall.SIGKILL); err != nil {
			log.Println("failed to kill: ", err)
		}
	}()
	if !testWork {
		err := os.Chmod(testDir, 0777)
		check(err)
		err = os.RemoveAll(testDir)
		check(err)
		return
	}
	log.SetOutput(os.Stderr)

}
