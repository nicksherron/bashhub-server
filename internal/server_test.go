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
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

var (
	noCleanup   = flag.Bool("no-cleanup", false, "don't remove testdata directory with sqlite db after test")
	postgres    = flag.Bool("postgres", false, "run postgres tests")
	postgresUri = flag.String("postgres-uri", "postgres://postgres:@localhost:5444?sslmode=disable", "postgres uri to use for postgres tests")
)

func TestMain(m *testing.M) {

	flag.Parse()
	dirCleanup()
	defer func() {
		if !*noCleanup {
			dirCleanup()
		}
	}()

	var err error
	dir, err = os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	testDir = os.TempDir()
	log.Println("test directory", testDir)
	DbPath = filepath.Join(testDir, "test.db")
	LogFile = filepath.Join(testDir, "server.log")
	log.Print("sqlite tests")
	router = setupRouter()
	m.Run()

	if *postgres {
		log.Print("postgres tests")
		DbPath = *postgresUri
		router = setupRouter()
		m.Run()
	}

}

func TestToken(t *testing.T) {
	createUser(t)
	sysRegistered = false
	jwtToken = getToken(t)

}

func TestCommandInsert(t *testing.T) {
	var commandTests = []Command{
		{ProcessId: 90226, ExitStatus: 0, Command: "cat foo.txt"},
		{ProcessId: 90226, ExitStatus: 0, Command: "ls"},
		{ProcessId: 90226, ExitStatus: 0, Command: "pwd"},
		{ProcessId: 90226, ExitStatus: 0, Command: "whoami"},
		{ProcessId: 90226, ExitStatus: 0, Command: "which cat"},
		{ProcessId: 90226, ExitStatus: 0, Command: "head foo.txt"},
		{ProcessId: 90226, ExitStatus: 0, Command: "sed 's/fooobaar/foobar/g' somefile.txt"},
		{ProcessId: 90226, ExitStatus: 0, Command: "curl google.com"},
		{ProcessId: 90226, ExitStatus: 0, Command: "file /dev/null"},
		{ProcessId: 90226, ExitStatus: 0, Command: "df -h"},
		{ProcessId: 90226, ExitStatus: 127, Command: "catt"},
		{ProcessId: 90226, ExitStatus: 127, Command: "cay"},
	}

	hourAgo := time.Now().UnixNano() - (1 * time.Hour).Nanoseconds()

	for i := 0; i < 5; i++ {
		for _, tc := range commandTests {
			uid, err := uuid.NewRandom()
			if err != nil {
				t.Fatal(err)
			}
			tc.Path = dir
			tc.Created = time.Now().Unix()
			tc.ProcessStartTime = hourAgo
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
		{query: fmt.Sprintf("path=%v&unique=true&systemName=%v&query=^curl", url.QueryEscape(dir), system), expect: 1},
		{query: fmt.Sprintf("path=%v&query=^curl&unique=true", url.QueryEscape(dir)), expect: 1},
		{query: fmt.Sprintf("systemName=%v&query=^curl", system), expect: 5},
		{query: fmt.Sprintf("path=%v&query=^curl", url.QueryEscape(dir)), expect: 5},
		{query: fmt.Sprintf("systemName=%v&unique=true", system), expect: 10},
		{query: fmt.Sprintf("path=%v&unique=true", url.QueryEscape(dir)), expect: 10},
		{query: fmt.Sprintf("path=%v", url.QueryEscape(dir)), expect: 50},
		{query: fmt.Sprintf("systemName=%v", system), expect: 50},
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
			assert.Contains(t, system, data[0].SystemName)
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
