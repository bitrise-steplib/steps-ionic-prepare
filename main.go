package main

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/bitrise-community/steps-ionic-archive/ionic"
	"github.com/bitrise-community/steps-ionic-archive/jsdependency"
	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-tools/go-steputils/stepconf"
	ver "github.com/hashicorp/go-version"
)

type config struct {
	Platform              string `env:"platform,opt[ios,android,'ios,android']"`
	IonicVersion          string `env:"ionic_version"`
	CordovaVersion        string `env:"cordova_version"`
	CordovaIosVersion     string `env:"cordova_ios_version"`
	CordovaAndroidVersion string `env:"cordova_android_version"`
	WorkDir               string `env:"workdir,dir"`

	Username string          `env:"ionic_username"`
	Password stepconf.Secret `env:"ionic_password"`
}

func (c config) getField(field string) string {
	return reflect.ValueOf(c).FieldByName(field).String()
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
	packageManager := jsdependency.DetectTool(workDir)
	log.Printf("Js package manager used: %s", packageManager)
	if cfg.CordovaVersion != "" {
		fmt.Println()
		log.Infof("Updating cordova version to: %s", cfg.CordovaVersion)
		if err := jsdependency.InstallGlobalDependency(packageManager, "cordova", cfg.CordovaVersion); err != nil {
			failf("Failed to update ionic/cordova versions, error; %s", err)
		}
	}
	if cfg.IonicVersion != "" {
		fmt.Println()
		log.Infof("Updating ionic version to: %s", cfg.IonicVersion)
		if err := jsdependency.InstallGlobalDependency(packageManager, "ionic", cfg.IonicVersion); err != nil {
			failf("Failed to update ionic/cordova versions, error; %s", err)
		}
	}

	// Print cordova and ionic version
	cordovaVer, err := ionic.CordovaVersion()
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
		if err := jsdependency.Add(packageManager, jsdependency.Local, "@ionic/cli-plugin-ionic-angular@latest", "@ionic/cli-plugin-cordova@latest"); err != nil {
			failf("command failed, error: %s", err)
		}
	}

	// ionic login
	if cfg.Username != "" && cfg.Password != "" {
		if err := ionic.LoginCommand(cfg.Username, string(cfg.Password)); err != nil {
			failf("ionic login failed, error: %s", err)
		}
	}

	ionicMajorVersion := ionicVer.Segments()[0]

	platforms := strings.Split(cfg.Platform, ",")
	for i, p := range platforms {
		platforms[i] = strings.TrimSpace(p)
	}
	sort.Strings(platforms)

	// ionic prepare
	fmt.Println()
	log.Infof("Restoring cordova platforms with ionic prepare.")

	if err := ionic.PrepareCommand(ionicMajorVersion); err != nil {
		failf("ionic prepare failed, error: %s", err)
	}
}
