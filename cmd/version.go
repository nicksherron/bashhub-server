/*
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
 */

package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var (
	GitCommit      string
	BuildDate  string
	Version    string
	OsArch     = fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH)
	GoVersion  = runtime.Version()
	versionCmd = &cobra.Command{

		Use:   "version",
		Short: "Print the version number and build info",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Build Date:", BuildDate)
			fmt.Println("Git Commit:", GitCommit)
			fmt.Println("Version:", Version)
			fmt.Println("Go Version:", GoVersion)
			fmt.Println("OS / Arch:", OsArch)
		},
	}
)

func init() {
	rootCmd.AddCommand(versionCmd)
}
