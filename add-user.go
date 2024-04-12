package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const downloadURL = "https://nextcloud.xxxxx.com/s/xxxx/xxxx/Pub-keys-users.txt"

func main() {
	// Check for the required username argument
	if len(os.Args) < 2 {
		fmt.Println("Usage: add-user <username>")
		return
	}
	username := os.Args[1]

	// Step 1: Download the file
	err := downloadFile("Pub-keys-users.txt", downloadURL)
	if err != nil {
		fmt.Println("Error downloading the file:", err)
		return
	}

	defer func() {
		if err := os.Remove("Pub-keys-users.txt"); err != nil {
			fmt.Println("Error removing Pub-keys-users.txt:", err)
		} else {
			fmt.Println("Pub-keys-users.txt removed successfully.")
		}
	}()

	// Step 2 & 3: Search for the user and get the public key
	publicKey, found := searchUserAndGetPublicKey("Pub-keys-users.txt", username)
	if !found {
		fmt.Println("User not found. Please update Pub-keys-users.txt.")
		return
	}

	// Step 4: Create the user and set up their SSH public key
	err = createUserAndSetupSSH(username, publicKey)
	if err != nil {
		fmt.Println("Error setting up user:", err)
		return
	}

	fmt.Println("User setup successfully.")
}

// downloadFile downloads a file from the specified URL and saves it to the local file system.
func downloadFile(filepath, url string) error {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// searchUserAndGetPublicKey searches for the username in the file and returns the associated public key.
func searchUserAndGetPublicKey(filepath, username string) (string, bool) {
	file, err := os.Open(filepath)
	if err != nil {
		fmt.Println("Error opening the file:", err)
		return "", false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, username) {
			parts := strings.Split(line, ";;;")
			if len(parts) >= 3 {
				return parts[2], true // Assuming the public key is the second part
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading the file:", err)
	}
	return "", false
}

// createUserAndSetupSSH creates a new system user and sets up their SSH public key.
func createUserAndSetupSSH(username, publicKey string) error {
	// Create the user
	// cmd := exec.Command("useradd", "-m", username)
	// if err := cmd.Run(); err != nil {
	// 	return err
	// }

	// Attempt to create the user with useradd
	err := exec.Command("useradd", "-m", username).Run()
	if err != nil {
		// If useradd fails because the command is not found, try adduser
		if strings.Contains(err.Error(), "executable file not found") {
			err = exec.Command("adduser", "-D", username).Run() // -D option is used in Alpine for default behavior
			if err != nil {
				return err // Return error if adduser also fails
			}
		} else {
			return err // Return the original error if it's not due to useradd being missing
		}
	}

	defaultPassword := "1234qwer!"
	cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s:%s' | chpasswd", username, defaultPassword))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set password for user %s: %v", username, err)
	}

	// Create the .ssh directory and authorized_keys file
	homeDir := "/home/" + username
	sshDir := homeDir + "/.ssh"
	authKeysFile := sshDir + "/authorized_keys"

	if err := os.Mkdir(sshDir, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(authKeysFile, []byte(publicKey+"\n"), 0400); err != nil {
		return err
	}

	// Get UID and GID of the user
	uid, gid, err := getUserIDs(username)
	if err != nil {
		return fmt.Errorf("failed to get UID and GID for user %s: %v", username, err)
	}

	// Change ownership of the .ssh directory and authorized_keys file to the user
	if err := os.Chown(sshDir, uid, gid); err != nil {
		return err
	}
	if err := os.Chown(authKeysFile, uid, gid); err != nil {
		return err
	}

	return nil
}

func getUserIDs(username string) (int, int, error) {
	cmd := exec.Command("id", "-u", username)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0, 0, err
	}
	uidStr := strings.TrimSpace(out.String())
	uid, err := strconv.Atoi(uidStr)
	if err != nil {
		return 0, 0, err
	}

	cmd = exec.Command("id", "-g", username)
	out.Reset() // Clear the buffer for the next command
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0, 0, err
	}
	gidStr := strings.TrimSpace(out.String())
	gid, err := strconv.Atoi(gidStr)
	if err != nil {
		return 0, 0, err
	}

	return uid, gid, nil
}
