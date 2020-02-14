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
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

var (
	dir           string
	router        *gin.Engine
	sysRegistered bool
	jwtToken      string
	testDir string
)

const (
	user    = "tester"
	pass    = "tester"
	mac     = "888888888888888"
	email   = "test@email.com"
	system  = "system"
)

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
		"Username": user,
		"password": pass,
		"email":    email,
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
		"username": user,
		"password": pass,
		"mac":      mac,
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

	jwtToken = token
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

	w := testRequest("POST", "/api/v1/system", body)
	assert.Equal(t, 201, w.Code)

	sysRegistered = true

	return getToken(t)

}

func dirCleanup() {
	dbFiles := []string{
		"test.db", "test.db-shm", "test.db-wal",
	}
	for _, d := range dbFiles {
		err := os.RemoveAll(filepath.Join([]string{testDir, d}...))
		check(err)
	}
}

