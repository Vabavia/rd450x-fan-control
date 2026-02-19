# RD450X Fan Control

A lightweight Go utility for manual and automated fan speed control on Lenovo RD450X servers using IPMI OEM commands.

## Features
- Real-time Monitoring: Full dashboard with RPM and PWM duty cycle readings.
- Thermal Stats: Displays CPU, PCH, and VR temperatures plus Airflow rate.
- Granular Control: Set speed for individual fans or all fans simultaneously.

## Installation
Ensure ipmitool is installed on your Proxmox/Linux system:  
```bash
apt update && apt install ipmitool -y
```

## Usage

### Display Status Dashboard
Shows all 6 fans (RPM/PWM) and thermal sensors in a formatted ASCII table.  
```bash
./rd450x-fan-control status
```

Get full telemetry in JSON format for automation.
```bash
./rd450x-fan-control status --json
```

Example Output:
```json
{
  "fans": [
    { "name": "System Fan1", "rpm": "1558 RPM", "pwm": "50%" },
    { "name": "System Fan2", "rpm": "1476 RPM", "pwm": "70%" }
  ],
  "thermals": [
    { "name": "CPU1 Temp", "value": "35 °C" },
    { "name": "Airflow rate", "value": "11 CFM" }
  ]
}
```

### Get Specific Fan PWM
Instant check for a specific fan (01-06) using fast OEM read commands.  
```bash
./rd450x-fan-control get 01
```


### Set Fan Speed
Set a specific fan or all fans to a specific percentage (0-100).
```bash
# Set specific fan (ID 01 to 06)
./rd450x-fan-control set 01 45
```
```bash
# Set all fans at once
./rd450x-fan-control set all 30
```

## Safety Warning
Disclaimer: This tool uses raw IPMI OEM commands (0x2e 0x30 / 0x2e 0x31). Use at your own risk.  
Always monitor your hardware temperatures closely when reducing fan speeds below default levels.

## License
MIT