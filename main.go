package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/bitrise-community/steps-ionic-prepare/cordova"
	"github.com/bitrise-community/steps-ionic-prepare/ionic"
	"github.com/bitrise-community/steps-ionic-prepare/npm"
	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-tools/go-steputils/stepconf"
	ver "github.com/hashicorp/go-version"
)

type config struct {
	Platform              string `env:"platform,opt[ios,android,'ios,android']"`
	Readd                 bool   `env:"readd_platform,opt[true,false]"`
	IonicVersion          string `env:"ionic_version"`
	CordovaVersion        string `env:"cordova_version"`
	CordovaIosVersion     string `env:"cordova_ios_version"`
	CordovaAndroidVersion string `env:"cordova_android_version"`
	WorkDir               string `env:"workdir,dir"`
}

func (c config) getField(field string) string {
	r := reflect.ValueOf(c)
	f := reflect.Indirect(r).FieldByName(field)
	return string(f.String())
}

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}

func main() {
	var cfg config
	if err := stepconf.Parse(&cfg); err != nil {
		failf("Error: %s\n", err)
	}
	stepconf.Print(cfg)

	// Change dir to working directory
	workDir, err := pathutil.AbsPath(cfg.WorkDir)
	if err != nil {
		failf("Failed to expand WorkDir (%s), error: %s", cfg.WorkDir, err)
	}

	currentDir, err := pathutil.CurrentWorkingDirectoryAbsolutePath()
	if err != nil {
		failf("Failed to get current directory, error: %s", err)
	}

	if workDir != currentDir {
		fmt.Println()
		log.Infof("Switch working directory to: %s", workDir)

		revokeFunc, err := pathutil.RevokableChangeDir(workDir)
		if err != nil {
			failf("Failed to change working directory, error: %s", err)
		}
		defer func() {
			fmt.Println()
			log.Infof("Reset working directory")
			if err := revokeFunc(); err != nil {
				failf("Failed to reset working directory, error: %s", err)
			}
		}()
	}

	// Update cordova and ionic version
	if cfg.CordovaVersion != "" {
		fmt.Println()
		log.Infof("Updating cordova version to: %s", cfg.CordovaVersion)

		if err := npm.Remove(false, "cordova"); err != nil {
			failf("Failed to remove cordova, error: %s", err)
		}

		if err := npm.Install(true, "cordova@"+cfg.CordovaVersion); err != nil {
			failf("Failed to install cordova, error: %s", err)
		}
	}

	if cfg.IonicVersion != "" {
		fmt.Println()
		log.Infof("Updating ionic version to: %s", cfg.IonicVersion)

		if err := npm.Remove(false, "ionic"); err != nil {
			failf("Failed to remove ionic, error: %s", err)
		}

		if err := npm.Install(true, "ionic@"+cfg.IonicVersion); err != nil {
			failf("Failed to install ionic, error: %s", err)
		}

		fmt.Println()
		log.Infof("Installing local ionic cli")
		if err := npm.Install(false, "ionic@"+cfg.IonicVersion); err != nil {
			failf("command failed, error: %s", err)
		}
	}

	// Print cordova and ionic version
	cordovaVer, err := cordova.Version()
	if err != nil {
		failf("Failed to get cordova version, error: %s", err)
	}

	fmt.Println()
	log.Printf("cordova version: %s", colorstring.Green(cordovaVer.String()))

	ionicVer, err := ionic.Version()
	if err != nil {
		failf("Failed to get ionic version, error: %s", err)
	}

	log.Printf("ionic version: %s", colorstring.Green(ionicVer.String()))

	// Ionic CLI plugins angular and cordova have been marked as deprecated for
	// version 3.8.0 and above.
	ionicVerConstraint, err := ver.NewConstraint("< 3.8.0")
	if err != nil {
		failf("Could not create version constraint for ionic: %s", err)
	}
	if ionicVerConstraint.Check(ionicVer) {
		fmt.Println()
		log.Infof("Installing cordova and angular plugins")
		if err := npm.Install(false, "@ionic/cli-plugin-ionic-angular@latest", "@ionic/cli-plugin-cordova@latest"); err != nil {
			failf("command failed, error: %s", err)
		}
	}

	ionicMajorVersion := ionicVer.Segments()[0]

	platforms := strings.Split(cfg.Platform, ",")

	// ionic prepare
	fmt.Println()
	log.Infof("Building project")

	// platform rm
	if cfg.Readd {
		for _, platform := range platforms {
			cmdArgs := []string{"ionic"}
			if ionicMajorVersion > 2 {
				cmdArgs = append(cmdArgs, "cordova")
			}

			cmdArgs = append(cmdArgs, "platform", "rm")

			cmdArgs = append(cmdArgs, platform)

			cmd := command.New(cmdArgs[0], cmdArgs[1:]...)
			cmd.SetStdout(os.Stdout).SetStderr(os.Stderr).SetStdin(strings.NewReader("y"))

			log.Donef("$ %s", cmd.PrintableCommandArgs())

			if err := cmd.Run(); err != nil {
				failf("command failed, error: %s", err)
			}
		}
	}

	{
		// platform add
		for _, platform := range platforms {
			cmdArgs := []string{"ionic"}
			if ionicMajorVersion > 2 {
				cmdArgs = append(cmdArgs, "cordova")
			}

			cmdArgs = append(cmdArgs, "platform", "add")

			platformVersion := platform
			pv := cfg.getField("Cordova" + strings.Title(platform) + "Version")
			if pv == "master" {
				platformVersion = "https://github.com/apache/cordova-" + platform + ".git"
			} else if pv != "" {
				platformVersion = platform + "@" + pv
			}

			cmdArgs = append(cmdArgs, platformVersion)

			cmd := command.New(cmdArgs[0], cmdArgs[1:]...)
			cmd.SetStdout(os.Stdout).SetStderr(os.Stderr).SetStdin(strings.NewReader("y"))

			log.Donef("$ %s", cmd.PrintableCommandArgs())

			if err := cmd.Run(); err != nil {
				failf("command failed, error: %s", err)
			}
		}
	}
}
