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
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"
)

type cList struct {
	UUID    string `json:"uuid"`
	Command string `json:"command"`
	Created int64  `json:"created"`
}

type commandsList []cList

var (
	barTemplate   = `{{string . "message"}}{{counters . }} {{bar . }} {{percent . }} {{speed . "%s inserts/sec" }}`
	bar           *pb.ProgressBar
	progress      bool
	srcUser       string
	dstUser       string
	srcURL        string
	dstURL        string
	srcPass       string
	dstPass       string
	srcToken      string
	dstToken      string
	sysRegistered bool
	workers       int
	wg            sync.WaitGroup
	cmdList       commandsList
	transferCmd   = &cobra.Command{
		Use:   "transfer",
		Short: "transfer bashhub history ",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Flags().Parse(args)
			sysRegistered = false
			srcToken = getToken(srcURL, srcUser, srcPass)
			sysRegistered = false
			dstToken = getToken(dstURL, dstUser, dstPass)
			cmdList = getCommandList()
			counter := 0
			if !progress {
				bar = pb.ProgressBarTemplate(barTemplate).Start(len(cmdList)).SetMaxWidth(70)
				bar.Set("message", "inserting records \t")
			}
			client := &http.Client{}
			for _, v := range cmdList {
				//commandLookup(v.UUID, client)
				//}
				wg.Add(1)
				counter++
				go func(c cList) {
					defer wg.Done()
					commandLookup(c.UUID, client)
				}(v)
				if counter > workers {
					wg.Wait()
					counter = 0
				}
			}
			wg.Wait()
		},
	}
)

func init() {
	rootCmd.AddCommand(transferCmd)
	transferCmd.PersistentFlags().StringVar(&srcURL, "src-url", "https://bashhub.com", "source url")
	transferCmd.PersistentFlags().StringVar(&srcUser, "src-user", "", "source username")
	transferCmd.PersistentFlags().StringVar(&srcPass, "src-pass", "", "source password")
	transferCmd.PersistentFlags().StringVar(&dstURL, "dst-url", "http://localhost:8080", "destination url")
	transferCmd.PersistentFlags().StringVar(&dstUser, "dst-user", "", "destination username")
	transferCmd.PersistentFlags().StringVar(&dstPass, "dst-pass", "", "destination password")
	transferCmd.PersistentFlags().BoolVarP(&progress, "progress", "p", false, "show progress bar")
	transferCmd.PersistentFlags().IntVarP(&workers, "workers", "w", 10, "max number of concurrent requests")
}

func sysRegister(mac string, site string, user string, pass string) string {

	var token string
	func() {
		var null *string
		auth := map[string]interface{}{
			"username": user,
			"password": pass,
			"mac":      null,
		}

		payloadBytes, err := json.Marshal(auth)
		if err != nil {
			log.Fatal(err)
		}

		body := bytes.NewReader(payloadBytes)

		u := fmt.Sprintf("%v/api/v1/login", site)
		req, err := http.NewRequest("POST", u, body)
		if err != nil {
			log.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()

		buf, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			log.Fatal(err)
		}
		j := make(map[string]interface{})

		json.Unmarshal(buf, &j)

		if len(j) == 0 {
			log.Fatal("login failed for ", site)

		}
		token = fmt.Sprintf("Bearer %v", j["accessToken"])

	}()

	host, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}
	sys := map[string]interface{}{
		"clientVersion": "1.2.0",
		"name":          "migration",
		"hostname":      host,
		"mac":           mac,
	}
	payloadBytes, err := json.Marshal(sys)
	if err != nil {
		log.Fatal(err)
	}

	body := bytes.NewReader(payloadBytes)

	u := fmt.Sprintf("%v/api/v1/system", srcURL)
	req, err := http.NewRequest("POST", u, body)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	log.Println(resp.StatusCode)
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(string(b))

	sysRegistered = true
	return getToken(site, user, pass)

}

func getToken(site string, user string, pass string) string {
	// function used by bashhub to identify system
	cmd := exec.Command("python", "-c", "import uuid; print(str(uuid.getnode()))")
	m, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	mac := strings.ReplaceAll(string(m), "\n", ``)
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

	u := fmt.Sprintf("%v/api/v1/login", site)
	req, err := http.NewRequest("POST", u, body)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 409 && !sysRegistered {
		//	register system
		return sysRegister(mac, site, user, pass)
	}
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	j := make(map[string]interface{})

	json.Unmarshal(buf, &j)

	if len(j) == 0 {
		log.Fatal("login failed for ", site)

	}
	return fmt.Sprintf("Bearer %v", j["accessToken"])
}

func getCommandList() commandsList {
	u := strings.TrimSpace(srcURL) + "/api/v1/command/search?unique=true&limit=1000000"
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Add("Authorization", srcToken)

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		log.Println("Error on response.\n", err)
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Fatalf("failed to get command list from %v, go status code %v", srcURL, resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	var result commandsList
	json.Unmarshal(body, &result)

	return result
}

func commandLookup(uuid string, client *http.Client) {
	u := strings.TrimSpace(srcURL) + "/api/v1/command/" + strings.TrimSpace(uuid)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Add("Authorization", srcToken)

	resp, err := client.Do(req)

	if err != nil {
		log.Println("Error on response.\n", err)
	}

	//defer func() {
	//	err = resp.Body.Close()
	//	if err !=  nil {
	//		log.Println(err)
	//	}
	//
	//}()
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Fatalf("failed command lookup from %v, go status code %v", srcURL, resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	srcSend(body, client)
}

func srcSend(data []byte, client *http.Client) {
	defer func() {
		if !progress {
			bar.Add(1)
		}
	}()
	body := bytes.NewReader(data)

	u := dstURL + "/api/v1/import"
	req, err := http.NewRequest("POST", u, body)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("Authorization", dstToken)
	resp, err := client.Do(req)

	if err != nil {
		log.Println("Error on response.\n", err)
	}

	defer resp.Body.Close()
}
