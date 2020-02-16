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

package internal

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

var (
	testWork         = flag.Bool("testwork", false, "don't remove sqlite db and server log when done and print location")
	postgres         = flag.String("postgres-uri", "", "postgres uri to use for postgres tests")
	sessionStartTime int64
	pid              string
	dir              string
	router           *gin.Engine
	sysRegistered    bool
	jwtToken         string
	testDir          string
	system           sysStruct
)

type sysStruct struct {
	user       string
	pass       string
	mac        int
	email      string
	systemName string
	host       string
}

func TestMain(m *testing.M) {

	flag.Parse()
	defer dirCleanup()

	var err error
	testDir, err = ioutil.TempDir("", "bashhub-server-test-")
	check(err)
	dir = "/tmp/foo"

	dbPath := filepath.Join(testDir, "test.db")
	logFile := filepath.Join(testDir, "server.log")
	log.Print("sqlite tests")
	router = setupRouter(dbPath, logFile)

	system = sysStruct{
		user:  "tester",
		pass:  "tester",
		mac:   888888888888888,
		email: "test@email.com",
		host:  "some-host",
	}
	m.Run()

	if *postgres != "" {
		log.Print("postgres tests")
		dbPath := *postgres
		logFile := filepath.Join(testDir, "postgres-server.log")
		router = setupRouter(dbPath, logFile)
		m.Run()
	}

}

func testRequest(method string, u string, body io.Reader) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, u, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", jwtToken)
	router.ServeHTTP(w, req)
	return w
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func createUser(t *testing.T) {
	auth := map[string]interface{}{
		"email":    system.email,
		"Username": system.user,
		"password": system.pass,
	}

	payloadBytes, err := json.Marshal(auth)
	if err != nil {
		log.Fatal(err)
	}
	body := bytes.NewReader(payloadBytes)
	w := testRequest("POST", "/api/v1/user", body)
	assert.Equal(t, 200, w.Code)
}

func getToken(t *testing.T) string {

	auth := map[string]interface{}{
		"username": system.user,
		"password": system.pass,
		"mac":      strconv.Itoa(system.mac),
	}

	payloadBytes, err := json.Marshal(auth)
	if err != nil {
		log.Fatal(err)
	}

	body := bytes.NewReader(payloadBytes)
	w := testRequest("POST", "/api/v1/login", body)
	assert.Equal(t, 200, w.Code)

	buf, err := ioutil.ReadAll(w.Body)

	if err != nil {
		t.Fatal(err)
	}
	j := make(map[string]interface{})

	err = json.Unmarshal(buf, &j)
	check(err)

	if len(j) == 0 {
		t.Fatal("login failed for  getToken")

	}
	token := fmt.Sprintf("Bearer %v", j["accessToken"])

	if !sysRegistered {
		//	register system
		return sysRegister(t, token)
	}
	return token
}

func sysRegister(t *testing.T, token string) string {

	jwtToken = token
	sysPayload := map[string]interface{}{
		"clientVersion": "1.2.0",
		"name":          system.systemName,
		"hostname":      system.host,
		"mac":           strconv.Itoa(system.mac),
	}
	payloadBytes, err := json.Marshal(sysPayload)
	check(err)

	body := bytes.NewReader(payloadBytes)

	w := testRequest("POST", "/api/v1/system", body)
	assert.Equal(t, 201, w.Code)

	sysRegistered = true

	return getToken(t)

}

func TestToken(t *testing.T) {
	createUser(t)
	systems := []string{
		"system-1",
		"system-2",
		"system-3",
	}
	for _, sys := range systems {
		system.systemName = sys
		system.mac++
		sysRegistered = false
		jwtToken = getToken(t)
	}

}

func TestCommandInsert(t *testing.T) {
	var commandTests = []Command{
		{ExitStatus: 0, Command: "cat foo.txt"},
		{ExitStatus: 0, Command: "ls"},
		{ExitStatus: 0, Command: "pwd"},
		{ExitStatus: 0, Command: "whoami"},
		{ExitStatus: 0, Command: "which cat"},
		{ExitStatus: 0, Command: "head foo.txt"},
		{ExitStatus: 0, Command: "sed 's/fooobaar/foobar/g' somefile.txt"},
		{ExitStatus: 0, Command: "curl google.com"},
		{ExitStatus: 0, Command: "file /dev/null"},
		{ExitStatus: 0, Command: "df -h"},
		{ExitStatus: 127, Command: "catt"},
		{ExitStatus: 127, Command: "cay"},
	}

	sessionStartTime = time.Now().Unix() * 1000
	for i := 0; i < 5; i++ {
		for _, tc := range commandTests {
			uid, err := uuid.NewRandom()
			if err != nil {
				t.Fatal(err)
			}
			tc.ProcessId = i
			tc.Path = dir
			tc.Created = time.Now().Unix() * 1000
			tc.ProcessStartTime = sessionStartTime
			tc.Uuid = uid.String()
			payloadBytes, err := json.Marshal(&tc)
			if err != nil {
				t.Fatal(err)
			}
			body := bytes.NewReader(payloadBytes)
			w := testRequest("POST", "/api/v1/command", body)
			assert.Equal(t, 200, w.Code)
		}

	}
}

func TestCommandQuery(t *testing.T) {
	type queryTest struct {
		query  string
		expect int
	}
	var queryTests = []queryTest{
		{query: fmt.Sprintf("path=%v&unique=true&systemName=%v&query=^curl", url.QueryEscape(dir), system.systemName), expect: 1},
		{query: fmt.Sprintf("path=%v&query=^curl&unique=true", url.QueryEscape(dir)), expect: 1},
		{query: fmt.Sprintf("systemName=%v&query=^curl", system.systemName), expect: 5},
		{query: fmt.Sprintf("path=%v&query=^curl", url.QueryEscape(dir)), expect: 5},
		{query: fmt.Sprintf("systemName=%v&unique=true", system.systemName), expect: 10},
		{query: fmt.Sprintf("path=%v&unique=true", url.QueryEscape(dir)), expect: 10},
		{query: fmt.Sprintf("path=%v", url.QueryEscape(dir)), expect: 50},
		{query: fmt.Sprintf("systemName=%v", system.systemName), expect: 50},
		{query: "query=^curl&unique=true", expect: 1},
		{query: "query=^curl", expect: 5},
		{query: "unique=true", expect: 10},
		{query: "limit=1", expect: 1},
	}

	for _, v := range queryTests {
		func() {
			u := fmt.Sprintf("/api/v1/command/search?%v", v.query)
			w := testRequest("GET", u, nil)
			assert.Equal(t, 200, w.Code)
			b, err := ioutil.ReadAll(w.Body)
			if err != nil {
				t.Fatal(err)
			}
			var data []Query
			err = json.Unmarshal(b, &data)
			if err != nil {
				t.Fatal(err)
			}

			if v.expect != len(data) {
				t.Fatalf("expected: %v, got: %v -- query: %v ", v.expect, len(data), v.query)
			}
			assert.Contains(t, system.systemName, data[0].SystemName)
			assert.Contains(t, dir, data[0].Path)
		}()
	}

}

func TestCommandFindDelete(t *testing.T) {

	var record Command

	func() {
		v := url.Values{}
		v.Add("limit", "1")
		v.Add("unique", "true")
		u := fmt.Sprintf("/api/v1/command/search?%v", v.Encode())
		w := testRequest("GET", u, nil)
		assert.Equal(t, 200, w.Code)
		b, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		var data []Command
		err = json.Unmarshal(b, &data)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 1, len(data))
		record = data[0]
	}()
	func() {
		u := fmt.Sprintf("/api/v1/command/%v", record.Uuid)
		w := testRequest("GET", u, nil)
		assert.Equal(t, 200, w.Code)
		b, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		var data Command
		err = json.Unmarshal(b, &data)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, record.Uuid, data.Uuid)
		pid = data.SessionID
	}()

	func() {
		u := fmt.Sprintf("/api/v1/command/%v", record.Uuid)
		w := testRequest("DELETE", u, nil)
		assert.Equal(t, 200, w.Code)
	}()
	func() {
		w := testRequest("GET", "/api/v1/command/search?", nil)
		assert.Equal(t, 200, w.Code)
		b, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		var data []Command
		err = json.Unmarshal(b, &data)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 49, len(data))
	}()

}

func TestStatus(t *testing.T) {
	u := fmt.Sprintf("/api/v1/client-view/status?processId=%v&startTime=%v", pid, sessionStartTime)
	w := testRequest("GET", u, nil)
	assert.Equal(t, 200, w.Code)
	b, err := ioutil.ReadAll(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	var status Status
	err = json.Unmarshal(b, &status)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, status.TotalCommands, 49)
	assert.Equal(t, status.TotalSessions, 5)
	assert.Equal(t, status.TotalSystems, 3)
	assert.Equal(t, status.TotalCommandsToday, 49)
	assert.Equal(t, status.SessionTotalCommands, 9)

}

func dirCleanup() {
	if !*testWork {
		err := os.Chmod(testDir, 0777)
		check(err)
		err = os.RemoveAll(testDir)
		check(err)
		return
	}
	log.Println("TESTWORK=", testDir)

}
