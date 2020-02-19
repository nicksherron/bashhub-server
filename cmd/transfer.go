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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

type cList struct {
	Retries int
	UUID    string `json:"uuid"`
	Command string `json:"command"`
	Created int64  `json:"created"`
}

type commandsList []cList

var (
	barTemplate   = `{{string . "message" | green }}{{counters . }} {{bar . }} {{percent . }} {{speed . "%s inserts/sec" | green}}`
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
	unique        bool
	limit         int
	dstCounter    uint64
	srcCounter    uint64
	inserted      uint64
	wgSrc         sync.WaitGroup
	wgDst         sync.WaitGroup
	cmdList       commandsList

	transferCmd = &cobra.Command{
		Use:   "transfer",
		Short: "Transfer bashhub history from one server to another",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Flags().Parse(args)
			switch {
			case srcUser == "":
				_ = cmd.Usage()
				fmt.Print("\n\n")
				log.Fatal("src-user can't be blank")
			case dstUser == "":
				_ = cmd.Usage()
				fmt.Print("\n\n")
				log.Fatal("--dst-user can't be blank")
			case srcPass == "" || dstPass == "":
				if srcPass == "" {
					srcPass = credentials("source")
				}
				if dstPass == "" {
					dstPass = credentials("destination")
				}

			}

			if workers > 10 && srcURL == "https://bashhub.com" {
				msg := fmt.Sprintf(`
	WARNING: errors are likely to occur when setting workers higher
	than 10 when transferring from https://bashhub.com`)
				fmt.Print(msg, "\n\n")
			}
			run()
		},
	}
)

func init() {
	rootCmd.AddCommand(transferCmd)
	transferCmd.PersistentFlags().StringVar(&srcURL, "src-url", "https://bashhub.com", "source url ")
	transferCmd.PersistentFlags().StringVar(&srcUser, "src-user", "", "source username")
	transferCmd.PersistentFlags().StringVar(&srcPass, "src-pass", "", "source password (default is password prompt)")
	transferCmd.PersistentFlags().StringVar(&dstURL, "dst-url", "http://localhost:8080", "destination url")
	transferCmd.PersistentFlags().StringVar(&dstUser, "dst-user", "", "destination username")
	transferCmd.PersistentFlags().StringVar(&dstPass, "dst-pass", "", "destination password (default is password prompt)")
	transferCmd.PersistentFlags().BoolVarP(&progress, "quiet", "q", false, "don't show progress bar")
	transferCmd.PersistentFlags().IntVarP(&workers, "workers", "w", 10, "max number of concurrent requests")
	transferCmd.PersistentFlags().BoolVarP(&unique, "unique", "u", true, "don't include duplicate commands")
	transferCmd.PersistentFlags().IntVarP(&limit, "number", "n", 10000, "limit number of commands to transfer")

}
func credentials(s string) string {

	fmt.Printf("\nEnter %s password: ", s)
	bytePassword, err := terminal.ReadPassword(0)
	if err != nil {
		check(err)
	}
	password := string(bytePassword)

	return strings.TrimSpace(password)
}
func run() {
	sysRegistered = false
	srcToken = getToken(srcURL, srcUser, srcPass)
	sysRegistered = false
	dstToken = getToken(dstURL, dstUser, dstPass)
	cmdList = getCommandList()

	if !progress {
		bar = pb.ProgressBarTemplate(barTemplate).Start(len(cmdList)).SetMaxWidth(70)
		bar.Set("message", "transferring ")
	}
	fmt.Print("\nstarting transfer...\n\n")
	queue := make(chan cList, len(cmdList))
	pipe := make(chan []byte, len(cmdList))

	// ignore http errors. We try and recover them
	log.SetOutput(nil)
	go func() {
		for {
			select {
			case item := <-queue:
				wgDst.Add(1)
				atomic.AddUint64(&dstCounter, 1)
				go func(cmd cList) {
					defer wgDst.Done()
					cmd.commandLookup(pipe, queue)
				}(item)
				if atomic.CompareAndSwapUint64(&dstCounter, uint64(workers), 0) {
					wgDst.Wait()
				}
			case result := <-pipe:
				wgSrc.Add(1)
				atomic.AddUint64(&srcCounter, 1)
				go func(data []byte) {
					srcSend(data, 0)
				}(result)
				if atomic.CompareAndSwapUint64(&srcCounter, uint64(workers), 0) {
					wgSrc.Wait()
				}
			}
		}
	}()
	for _, v := range cmdList {
		v.Retries = 0
		queue <- v
	}

	for {
		if atomic.CompareAndSwapUint64(&inserted, uint64(len(cmdList)), 0) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !progress {
		bar.Finish()
	}
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

		err = json.Unmarshal(buf, &j)
		check(err)

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
		"name":          "transfer",
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

	sysRegistered = true
	return getToken(site, user, pass)

}

func getToken(site string, user string, pass string) string {
	mac := "888888888888888"
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

	err = json.Unmarshal(buf, &j)
	check(err)

	if len(j) == 0 || resp.StatusCode == 401 {
		log.Fatal("login failed for ", site)

	}
	return fmt.Sprintf("Bearer %v", j["accessToken"])
}

func getCommandList() commandsList {
	u := strings.TrimSpace(srcURL) + fmt.Sprintf("/api/v1/command/search?unique=%v&limit=%v", unique, limit)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Add("Authorization", srcToken)

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		log.Fatal("Error on response.\n", err)
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
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Fatal(err)
	}

	return result
}

func (item cList) commandLookup(pipe chan []byte, queue chan cList) {
	defer func() {
		if r := recover(); r != nil {
			mem := strings.Contains(fmt.Sprintf("%v", r), "runtime error: invalid memory address")
			eof := strings.Contains(fmt.Sprintf("%v", r), "EOF")
			if mem || eof {
				if item.Retries < 10 {
					item.Retries++
					queue <- item
					return
				} else {
					log.SetOutput(os.Stderr)
					log.Println("ERROR: failed over 10 times looking up command from source with uuid: ", item.UUID)
					log.SetOutput(nil)
				}
			} else {
				log.SetOutput(os.Stderr)
				log.Fatal(r)
			}
		}
	}()

	u := strings.TrimSpace(srcURL) + "/api/v1/command/" + strings.TrimSpace(item.UUID)
	req, err := http.NewRequest("GET", u, nil)

	if err != nil {
		panic(err)
	}
	req.Header.Add("Authorization", srcToken)

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("%v response from %v: %v", resp.StatusCode, srcURL, string(body))
		log.SetOutput(os.Stderr)
		log.Fatal(err)
	}
	pipe <- body
}

func srcSend(data []byte, retries int) {
	defer func() {
		if r := recover(); r != nil {
			retries++
			if retries < 10 {
				srcSend(data, retries)
				return
			}
			log.SetOutput(os.Stderr)
			log.Println("Error on response.\n", r)
			log.SetOutput(nil)
		}
		if !progress {
			bar.Add(1)
		}
		atomic.AddUint64(&inserted, 1)
		wgSrc.Done()
	}()

	body := bytes.NewReader(data)

	u := dstURL + "/api/v1/import"
	req, err := http.NewRequest("POST", u, body)
	if err != nil {
		log.SetOutput(os.Stderr)
		log.Fatal(err)
	}

	req.Header.Add("Authorization", dstToken)
	_, err = http.DefaultClient.Do(req)
	if err != nil {
		log.SetOutput(os.Stderr)
		log.Fatal(err)
	}
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
