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

package test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nicksherron/bashhub-server/internal"
	"github.com/stretchr/testify/assert"
)

var (
	dir           string
	router        *gin.Engine
	sysRegistered bool
	jwtToken      string
	db            = flag.String("db", sqliteDB(), "db path")
	postgres      = flag.Bool("postgres", false, "run postgres tests")
	postgresUri   = flag.String("postgres-uri", "postgres://postgres:@localhost:5444?sslmode=disable", "postgres uri to use for postgres tests")
)

const (
	system  = "system"
	user    = "tester"
	pass    = "tester"
	mac     = "888888888888888"
	email   = "test@email.com"
	testdir = "testdata"
)

func sqliteDB() string {
	return filepath.Join(dir, "testdata/test.db")
}
func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func createUser(t *testing.T) {
	auth := map[string]interface{}{
		"Username": user,
		"password": pass,
		"email":    email,
	}

	payloadBytes, err := json.Marshal(auth)
	if err != nil {
		log.Fatal(err)
	}
	body := bytes.NewReader(payloadBytes)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/user", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func getToken(t *testing.T) string {

	auth := map[string]interface{}{
		"username": user,
		"password": pass,
		"mac":      mac,
	}

	payloadBytes, err := json.Marshal(auth)
	if err != nil {
		log.Fatal(err)
	}

	body := bytes.NewReader(payloadBytes)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/login", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	buf, err := ioutil.ReadAll(w.Body)

	if err != nil {
		t.Fatal(err)
	}
	j := make(map[string]interface{})

	json.Unmarshal(buf, &j)

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

	host, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}
	sys := map[string]interface{}{
		"clientVersion": "1.2.0",
		"name":          system,
		"hostname":      host,
		"mac":           mac,
	}
	payloadBytes, err := json.Marshal(sys)
	check(err)

	body := bytes.NewReader(payloadBytes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/system", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", token)

	router.ServeHTTP(w, req)

	assert.Equal(t, 201, w.Code)

	sysRegistered = true

	return getToken(t)

}

func TestMain(m *testing.M) {

	flag.Parse()

	if *db == sqliteDB() {

		err := os.RemoveAll(testdir)
		if err != nil {
			log.Fatal(err)
		}

		err = os.Mkdir(testdir, 0700)
		if err != nil {
			log.Fatal(err)
		}
	}
	var err error
	dir, err = os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	internal.DbPath = *db
	router = internal.SetupRouter()
	m.Run()

	if *postgres {
		internal.DbPath = *postgresUri
		router = internal.SetupRouter()
		m.Run()
	}
}

func TestToken(t *testing.T) {

	w := httptest.NewRecorder()

	req, _ := http.NewRequest("GET", "/ping", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "{\"message\":\"pong\"}\n", w.Body.String())
	createUser(t)
	sysRegistered = false
	jwtToken = getToken(t)

}

var commandTests = []internal.Command{
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

func TestCommand(t *testing.T) {
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
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/command", body)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Add("Authorization", jwtToken)
			router.ServeHTTP(w, req)
			assert.Equal(t, 200, w.Code)
		}
	}

	var allQueries = map[string]string{
		"unique":     "true",
		"limit":      "1",
		"query":      "curl",
		"path":       dir,
		"systemName": system,
	}
	var queryTests []url.Values
	allQuery := url.Values{}

	for keyP, valP := range allQueries {
		allQuery.Add(keyP, valP)

		for kepC, valC := range allQueries {
			if keyP == kepC {
				continue
			}
			v := url.Values{}
			v.Add(kepC, valC)
			v.Add(keyP, valP)
			queryTests = append(queryTests, v)
		}

	}

	func() {
		w := httptest.NewRecorder()
		u := fmt.Sprintf("/api/v1/command/search?%v", allQuery.Encode())
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Add("Authorization", jwtToken)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	}()

	for _, v := range queryTests {
		func() {
			w := httptest.NewRecorder()
			u := fmt.Sprintf("/api/v1/command/search?%v", v.Encode())
			req, _ := http.NewRequest("GET", u, nil)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Add("Authorization", jwtToken)
			router.ServeHTTP(w, req)
			assert.Equal(t, 200, w.Code)
			b, err := ioutil.ReadAll(w.Body)
			if err != nil {
				t.Fatal(err)
			}
			var data []internal.Query
			err = json.Unmarshal(b, &data)
			if err != nil {
				t.Fatal(err)
			}

			assert.GreaterOrEqual(t, len(data), 1)
			assert.Contains(t, system, data[0].SystemName)
			assert.Contains(t, dir, data[0].Path)
		}()
	}



	func() {
		w := httptest.NewRecorder()
		v := url.Values{}
		v.Add("unique", "true")
		u := fmt.Sprintf("/api/v1/command/search?%v", v.Encode())
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Add("Authorization", jwtToken)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
		b, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		var data []internal.Command
		err = json.Unmarshal(b, &data)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 10, len(data))
	}()
	func() {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/command/search?", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Add("Authorization", jwtToken)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
		b, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		var data []internal.Command
		err = json.Unmarshal(b, &data)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 50, len(data))
	}()
	func() {
		w := httptest.NewRecorder()
		v := url.Values{}
		v.Add("query", "curl")
		v.Add("unique", "true")
		u := fmt.Sprintf("/api/v1/command/search?%v", v.Encode())
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Add("Authorization", jwtToken)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
		b, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		var data []internal.Command
		err = json.Unmarshal(b, &data)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 1, len(data))
	}()
	func() {
		w := httptest.NewRecorder()
		v := url.Values{}
		v.Add("unique", "true")
		v.Add("systemName", system)
		u := fmt.Sprintf("/api/v1/command/search?%v", v.Encode())
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Add("Authorization", jwtToken)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
		b, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		var data []internal.Command
		err = json.Unmarshal(b, &data)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 10, len(data))
	}()
	func() {
		w := httptest.NewRecorder()
		v := url.Values{}
		v.Add("path", dir)
		v.Add("unique", "true")
		u := fmt.Sprintf("/api/v1/command/search?%v", v.Encode())
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Add("Authorization", jwtToken)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
		b, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		var data []internal.Command
		err = json.Unmarshal(b, &data)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 10, len(data))
	}()
	var record internal.Command

	func() {
		w := httptest.NewRecorder()
		v := url.Values{}
		v.Add("limit","1")
		v.Add("unique", "true")
		u := fmt.Sprintf("/api/v1/command/search?%v", v.Encode())
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Add("Authorization", jwtToken)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
		b, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		var data []internal.Command
		err = json.Unmarshal(b, &data)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 1, len(data))
		record = data[0]
	}()
	func() {
		w := httptest.NewRecorder()
		u := fmt.Sprintf("/api/v1/command/%v", record.Uuid)
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Add("Authorization", jwtToken)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
		b, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		var data internal.Command
		err = json.Unmarshal(b, &data)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, record.Uuid, data.Uuid)
	}()
	func() {
		w := httptest.NewRecorder()
		u := fmt.Sprintf("/api/v1/command/%v", record.Uuid)
		req, _ := http.NewRequest("DELETE", u, nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Add("Authorization", jwtToken)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	}()
	func() {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/command/search?", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Add("Authorization", jwtToken)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
		b, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		var data []internal.Command
		err = json.Unmarshal(b, &data)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 49, len(data))
	}()
}
