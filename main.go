package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// FanStatus represents fan telemetry data
type FanStatus struct {
	Name string `json:"name"`
	RPM  string `json:"rpm"`
	PWM  string `json:"pwm"`
}

// ThermalStatus represents temperature and airflow data
type ThermalStatus struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// StatusReport is the combined object for JSON output
type StatusReport struct {
	Fans     []FanStatus     `json:"fans"`
	Thermals []ThermalStatus `json:"thermals"`
}

// ipmiRaw executes raw IPMI OEM commands
func ipmiRaw(args ...string) (string, error) {
	fullArgs := append([]string{"raw", "0x2e"}, args...)
	cmd := exec.Command("ipmitool", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, string(out))
	}
	return string(out), nil
}

// hexToPercent converts a hex string (e.g., "32") to a percentage string ("50%")
func hexToPercent(hexStr string) string {
	dec, err := strconv.ParseInt(hexStr, 16, 64)
	if err != nil {
		return "N/A"
	}
	return fmt.Sprintf("%d%%", dec)
}

// getPWMs reads PWM percentages via undocumented OEM command 0x31
func getPWMs() map[string]string {
	pwms := make(map[string]string)

	out, err := ipmiRaw("0x31")
	if err != nil {
		return pwms // Return empty map if command fails
	}

	parts := strings.Fields(out)
	if len(parts) >= 7 {
		pwms["System Fan1"] = hexToPercent(parts[1])
		pwms["System Fan2"] = hexToPercent(parts[2])
		pwms["System Fan3"] = hexToPercent(parts[3])
		pwms["System Fan4"] = hexToPercent(parts[4])
		pwms["CPU Fan1"] = hexToPercent(parts[5])
		pwms["CPU Fan2"] = hexToPercent(parts[6])
	}

	return pwms
}

// getSpeed instantly fetches the PWM for a specific fan
func getSpeed(idStr string) {
	if strings.ToLower(idStr) == "all" || idStr == "0" || idStr == "00" {
		fmt.Println("Error: To view all fans, use the 'status' command.")
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 || id > 6 {
		fmt.Println("Error: Fan ID must be between 01 and 06")
		return
	}

	out, err := ipmiRaw("0x31")
	if err != nil {
		fmt.Printf("IPMI Error: %v\n", err)
		os.Exit(1)
	}

	parts := strings.Fields(out)
	if len(parts) > id {
		pwm := hexToPercent(parts[id])
		fmt.Printf("Fan %02d PWM: %s\n", id, pwm)
	} else {
		fmt.Println("Error: Unexpected IPMI response format or missing data")
	}
}

// setSpeed handles the fan speed adjustment logic
func setSpeed(idStr, speedStr string) {
	speed, err := strconv.Atoi(speedStr)
	if err != nil || speed < 0 || speed > 100 {
		fmt.Println("Error: Speed must be an integer between 0 and 100")
		return
	}

	hexSpeed := fmt.Sprintf("0x%x", speed)

	formattedID := idStr
	isAll := false

	// Support for "all", "0", or "00" to set all fans at once
	if strings.ToLower(idStr) == "all" || idStr == "0" || idStr == "00" {
		formattedID = "00"
		isAll = true
	} else if len(idStr) == 1 {
		formattedID = "0" + idStr
	}

	_, err = ipmiRaw("0x30", "00", formattedID, hexSpeed)
	if err != nil {
		fmt.Printf("IPMI Error: %v\n", err)
		os.Exit(1)
	}

	if isAll {
		fmt.Printf("All fans successfully set to %d%%\n", speed)
	} else {
		fmt.Printf("Fan %s successfully set to %d%%\n", formattedID, speed)
	}
}

// getStatus fetches and displays full sensor telemetry
func getStatus(asJson bool) {
	pwms := getPWMs()

	cmd := exec.Command("ipmitool", "sensor", "list")
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error fetching sensor data: %v\n", err)
		return
	}

	lines := strings.Split(string(out), "\n")

	rpmMap := make(map[string]string)
	var thermals []ThermalStatus

	// 1. Parse sensor list to get RPMs and Thermals
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		unit := strings.TrimSpace(parts[2])
		lineUpper := strings.ToUpper(line)

		if val == "na" && !strings.Contains(lineUpper, "FAN") {
			continue // Skip disconnected thermal sensors
		}

		if strings.Contains(lineUpper, "FAN") && !strings.Contains(lineUpper, "POWER") {
			if val != "na" {
				rpmMap[name] = val + " RPM"
			}
		} else if strings.Contains(lineUpper, "TEMP") || strings.Contains(lineUpper, "AIRFLOW") {
			valFloat, err := strconv.ParseFloat(val, 64)
			cleanVal := val
			if err == nil {
				cleanVal = fmt.Sprintf("%.0f", valFloat)
			}

			cleanUnit := unit
			if cleanUnit == "degrees C" {
				cleanUnit = "°C"
			}

			if cleanVal == "0" && cleanUnit == "°C" {
				continue
			}

			thermals = append(thermals, ThermalStatus{
				Name:  name,
				Value: fmt.Sprintf("%s %s", cleanVal, cleanUnit),
			})
		}
	}

	// 2. Build the exact 6-fan array
	fanNames := []string{"System Fan1", "System Fan2", "System Fan3", "System Fan4", "CPU Fan1", "CPU Fan2"}
	var fans []FanStatus
	for _, name := range fanNames {
		pwm := pwms[name]
		if pwm == "" {
			pwm = "N/A"
		}
		rpm := rpmMap[name]
		if rpm == "" {
			rpm = "N/A"
		}
		fans = append(fans, FanStatus{
			Name: name,
			RPM:  rpm,
			PWM:  pwm,
		})
	}

	// Output results
	if asJson {
		report := StatusReport{Fans: fans, Thermals: thermals}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Printf("JSON Encoding Error: %v\n", err)
		}
	} else {
		// Table 1: Cooling System (Fans)
		fmt.Println("+----------------------+-----------------+---------+")
		fmt.Println("| COOLING SYSTEM                                   |")
		fmt.Println("+----------------------+-----------------+---------+")
		fmt.Printf("| %-20s | %-15s | %-7s |\n", "SENSOR", "RPM", "PWM (%)")
		fmt.Println("+----------------------+-----------------+---------+")
		for _, f := range fans {
			fmt.Printf("| %-20s | %-15s | %-7s |\n", f.Name, f.RPM, f.PWM)
		}

		// Transition to Table 2: Temperatures and Airflow
		fmt.Println("+----------------------+-----------------+---------+")
		fmt.Println("| TEMPERATURE & AIRFLOW                            |")
		fmt.Println("+----------------------+---------------------------+")
		fmt.Printf("| %-20s | %-25s |\n", "SENSOR", "VALUE")
		fmt.Println("+----------------------+---------------------------+")
		for _, t := range thermals {
			fmt.Printf("| %-20s | %-25s |\n", t.Name, t.Value)
		}
		fmt.Println("+----------------------+---------------------------+")
	}
}

// checkDependencies verifies if ipmitool is installed in PATH
func checkDependencies() error {
	_, err := exec.LookPath("ipmitool")
	return err
}

func main() {
	if err := checkDependencies(); err != nil {
		fmt.Println("Error: 'ipmitool' not found in PATH. Install it via: apt install ipmitool")
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  rd450x-fan-control status [--json]")
		fmt.Println("  rd450x-fan-control get <id>")
		fmt.Println("  rd450x-fan-control set <id|all> <speed>")
		return
	}

	switch os.Args[1] {
	case "status":
		isJson := len(os.Args) > 2 && os.Args[2] == "--json"
		getStatus(isJson)
	case "get":
		if len(os.Args) < 3 {
			fmt.Println("Error: Missing arguments. Example: rd450x-fan-control get 01")
			return
		}
		getSpeed(os.Args[2])
	case "set":
		if len(os.Args) < 4 {
			fmt.Println("Error: Missing arguments. Example: rd450x-fan-control set all 50")
			return
		}
		setSpeed(os.Args[2], os.Args[3])
	case "testrun":
		fmt.Println("--- STARTING AUTOMATED TEST SEQUENCE ---")

		fmt.Println("[STEP 0] Saving current fan speeds...")
		originalPWMs := getPWMs()

		// Map of fan names to their corresponding IDs for the 'set' command
		fanIDs := map[string]string{
			"System Fan1": "01",
			"System Fan2": "02",
			"System Fan3": "03",
			"System Fan4": "04",
			"CPU Fan1":    "05",
			"CPU Fan2":    "06",
		}

		// Save only the fans that returned a valid reading (not N/A)
		savedSpeeds := make(map[string]string)
		for name, id := range fanIDs {
			pwmStr := originalPWMs[name]
			if pwmStr != "N/A" && pwmStr != "" {
				// Strip the "%" sign to get the integer string (e.g., "50%" -> "50")
				speedVal := strings.TrimSuffix(pwmStr, "%")
				savedSpeeds[id] = speedVal
				fmt.Printf("    ID %s (%s) backed up at %s%%\n", id, name, speedVal)
			}
		}

		fmt.Println("\n[STEP 1] Testing 'set all' command (setting to 40%)...")
		setSpeed("all", "40")

		fmt.Println("\n[STEP 2] Testing 'get' command for Fan 01...")
		getSpeed("01")

		fmt.Println("\n[STEP 3] Displaying full status dashboard...")
		getStatus(false)

		fmt.Println("\n[STEP 4] Restoring original fan speeds...")
		for id, speed := range savedSpeeds {
			setSpeed(id, speed)
		}
		fmt.Println("\n--- TEST SEQUENCE COMPLETE ---")
	default:
		fmt.Printf("Error: Unknown command '%s'\n", os.Args[1])
	}
}
