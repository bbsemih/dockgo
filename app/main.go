//go:build linux
// +build linux

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// Usage: ./dockgo run <image> <command> <arg1> <arg2> ...
func main() {

	command := os.Args[3]
	args := os.Args[4:len(os.Args)]

	//Creating a temporary directory to serve as the chroot environment
	chrootDir, err := os.MkdirTemp("", "chroot")
	if err != nil {
		fmt.Printf("Error creating temporary directory: %s\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(chrootDir)

	//Copy the executable to the chroot directory
	//This is necessary because the the command being executed has access to its necessary executable within the chrooted environment.
	exePath, err := exec.LookPath(command)
	if err != nil {
		fmt.Printf("Error finding executable: %s\n", err)
		os.Exit(1)
	}
	exeName := filepath.Base(exePath)
	destPath := filepath.Join(chrootDir, exeName)
	if err := copyFile(exePath, destPath); err != nil {
		fmt.Printf("Error copying executable: %s\n", err)
		os.Exit(1)
	}

	//Change root directory using chroot
	if err := syscall.Chroot(chrootDir); err != nil {
		fmt.Printf("Error changing root directory: %s\n", err)
		os.Exit(1)
	}

	// Change the current working directory to the new root
	if err := syscall.Chdir("/"); err != nil {
		fmt.Printf("Error changing working directory: %s\n", err)
		os.Exit(1)
	}
	//Set up the /dev/null within the chroot environment
	devNull := filepath.Join(chrootDir, "dev", "null")
	if err := os.MkdirAll(filepath.Dir(devNull), 0755); err != nil {
		fmt.Printf("Error creating /dev/null: %s\n", err)
		os.Exit(1)
	}
	if err := ioutil.WriteFile(devNull, []byte{}, 0666); err != nil {
		fmt.Printf("Error writing to /dev/null: %s\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID,
	}

	err = cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode := exitError.ExitCode()
			fmt.Printf("Error: Command exited with code %d\n", exitCode)
			os.Exit(exitCode)
		}
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return out.Sync()
}
