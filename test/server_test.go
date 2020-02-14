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
	user          = "tester"
	pass          = "tester"
	mac           = "888888888888888"
)

func createUser(t *testing.T) {
	auth := map[string]interface{}{
		"Username": user,
		"password": pass,
		"email":    "test@email.com",
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
		"name":          "test-system",
		"hostname":      host,
		"mac":           mac,
	}
	payloadBytes, err := json.Marshal(sys)
	if err != nil {
		log.Fatal(err)
	}

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
	err := os.RemoveAll("testdata")
	if err != nil {
		log.Fatal(err)
	}
	dir, err = os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	err = os.Mkdir("testdata", 0700)
	if err != nil {
		log.Fatal(err)
	}

	internal.DbPath = filepath.Join(dir, "test.db")

	router = internal.SetupRouter()

	m.Run()
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
	func() {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/command/search?unique=true", nil)
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
		req, _ := http.NewRequest("GET", "/api/v1/command/search?query=%5Ecurl&unique=true", nil)
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
		req, _ := http.NewRequest("GET", "/api/v1/command/search?unique=true&systemName=test-system", nil)
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
		v.Add("unique", "true")
		v.Add("path", dir)
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
}
