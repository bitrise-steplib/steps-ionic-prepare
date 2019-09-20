package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/bitrise-community/steps-ionic-archive/ionic"
	"github.com/bitrise-community/steps-ionic-archive/jsdependency"
	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-tools/go-steputils/stepconf"
	ver "github.com/hashicorp/go-version"
)

type config struct {
	Platform       string `env:"platform,opt[ios,android,'ios,android']"`
	IonicVersion   string `env:"ionic_version"`
	CordovaVersion string `env:"cordova_version"`
	WorkDir        string `env:"workdir,dir"`

	Username string `env:"ionic_username"`
	Password string `env:"ionic_password"`

	UseCache bool `env:"cache_local_deps,opt[true,false]"`
}

func installDependency(packageManager jsdependency.Tool, name string, version string) error {
	fmt.Println()
	log.Infof("Updating %s version to: %s", name, version)
	cmdSlice, err := jsdependency.InstallGlobalDependencyCommand(packageManager, name, version)
	if err != nil {
		return fmt.Errorf("Failed to update %s version, error: %s", name, err)
	}
	for i, cmd := range cmdSlice {
		fmt.Println()
		log.Donef("$ %s", cmd.PrintableCommandArgs())
		fmt.Println()

		// Yarn returns an error if the package is not added before removal, ignoring
		if out, err := cmd.RunAndReturnTrimmedCombinedOutput(); err != nil && !(packageManager == jsdependency.Yarn && i == 0) {
			if errorutil.IsExitStatusError(err) {
				return fmt.Errorf("Failed to update %s version: %s failed, output: %s", name, cmd.PrintableCommandArgs(), out)
			}
			return fmt.Errorf("Failed to update %s version: %s failed, error: %s", name, cmd.PrintableCommandArgs(), err)
		}
	}
	return nil
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
	packageManager, err := jsdependency.DetectTool(workDir)
	if err != nil {
		log.Warnf("%s", err)
	}
	log.Printf("Js package manager used: %s", packageManager)
	if cfg.CordovaVersion != "" {
		if err := installDependency(packageManager, "cordova", cfg.CordovaVersion); err != nil {
			failf("%s", err)
		}
	}
	if cfg.IonicVersion != "" {
		if err := installDependency(packageManager, "ionic", cfg.IonicVersion); err != nil {
			failf("%s", err)
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
		cmd, err := jsdependency.AddCommand(packageManager, jsdependency.Local, "@ionic/cli-plugin-ionic-angular@latest", "@ionic/cli-plugin-cordova@latest")
		if err != nil {
			failf("%s", err)
		}
		fmt.Println()
		log.Donef("$ %s", cmd.PrintableCommandArgs())
		fmt.Println()

		if out, err := cmd.RunAndReturnTrimmedCombinedOutput(); err != nil {
			if errorutil.IsExitStatusError(err) {
				failf("Failed to install: %s failed, output: %s", cmd.PrintableCommandArgs(), out)
			}
			failf("Failed to install: %s failed, error: %s", cmd.PrintableCommandArgs(), err)
		}
	}

	// ionic login
	if cfg.Username != "" && cfg.Password != "" {
		if err := ionic.LoginCommand(cfg.Username, cfg.Password); err != nil {
			fmt.Println()
			log.Infof("Ionic login")

			cmd := ionic.LoginCommand(cfg.Username, cfg.Password)
			cmd.SetStdout(os.Stdout).SetStderr(os.Stderr).SetStdin(strings.NewReader("y"))

			log.Donef("$ ionic login *** ***")

			if err := cmd.Run(); err != nil {
				failf("ionic login command failed, error: %s", err)
			}
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

	cmd := ionic.PrepareCommand(ionicMajorVersion)
	cmd.SetStdout(os.Stdout).SetStderr(os.Stderr)

	log.Donef("$ %s", cmd.PrintableCommandArgs())

	if err := cmd.Run(); err != nil {
		failf("ionic prepare command %s failed, error: %s", cmd.PrintableCommandArgs(), err)
	}

	if cfg.UseCache {
		if err := cacheNpm(workDir); err != nil {
			log.Warnf("Failed to mark files for caching, error: %s", err)
		}
	}
}
