package main

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/oleiade/reflections"
	"github.com/sensu-community/sensu-plugin-sdk/sensu"
	"github.com/sensu/sensu-go/types"
	"github.com/shirou/gopsutil/v3/cpu"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	Critical   float64
	Warning    float64
	Interval   int
	UsageType  string
	UsageTypes string
}

type CpuStat struct {
	Total     float64 `json:"used"`
	User      float64 `json:"user"`
	System    float64 `json:"system"`
	Idle      float64 `json:"idle"`
	Nice      float64 `json:"nice"`
	Iowait    float64 `json:"iowait"`
	Irq       float64 `json:"irq"`
	Softirq   float64 `json:"softirq"`
	Steal     float64 `json:"steal"`
	Guest     float64 `json:"guest"`
	GuestNice float64 `json:"guestNice"`
}

type CpuCheck struct {
	Critical  float64
	Warning   float64
	UsageType string
	Inverted  bool
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-cpu-usage",
			Short:    "Check CPU usage and provide metrics",
			Keyspace: "sensu.io/plugins/check-cpu-usage/config",
		},
	}

	options = []*sensu.PluginConfigOption{
		{
			Path:      "critical",
			Argument:  "critical",
			Shorthand: "c",
			Default:   float64(90),
			Usage:     "Critical threshold for overall CPU usage",
			Value:     &plugin.Critical,
		},
		{
			Path:      "warning",
			Argument:  "warning",
			Shorthand: "w",
			Default:   float64(75),
			Usage:     "Warning threshold for overall CPU usage",
			Value:     &plugin.Warning,
		},
		{
			Path:      "sample-interval",
			Argument:  "sample-interval",
			Shorthand: "s",
			Default:   2,
			Usage:     "Length of sample interval in seconds",
			Value:     &plugin.Interval,
		},
		{
			Path:     "usage-type",
			Argument: "usage-type",
			Default:  "Total",
			Usage:    "Check cpu usage of type: ",
			Value:    &plugin.UsageType,
		},
		{
			Path:     "usage-types",
			Argument: "usage-types",
			Default:  "",
			Usage:    "Check cpu usage multiple type, see usage-type. list comma seperated format per value: 'type:warning:critical:invert'. Type is required, rest uses the defaults",
			Value:    &plugin.UsageTypes,
		},
	}

	fieldNames []string
)

func main() {
	fields := reflect.VisibleFields(reflect.TypeOf(struct{ cpu.TimesStat }{}))
	fieldNames = append(fieldNames, "Total")
	for _, field := range fields {
		// fmt.Printf("Key: %s\tType: %s\n", field.Name, field.Type)
		if field.Type.String() == "float64" {
			fieldNames = append(fieldNames, field.Name)
		}
	}
	options[3].Usage = options[3].Usage + fmt.Sprint(fieldNames)

	check := sensu.NewGoCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(event *types.Event) (int, error) {
	if plugin.Critical == 0 {
		return sensu.CheckStateWarning, fmt.Errorf("--critical is required")
	}
	if plugin.Warning == 0 {
		return sensu.CheckStateWarning, fmt.Errorf("--warning is required")
	}
	if plugin.Warning > plugin.Critical {
		return sensu.CheckStateWarning, fmt.Errorf("--warning cannot be greater than --critical")
	}
	if plugin.Interval == 0 {
		return sensu.CheckStateWarning, fmt.Errorf("--interval is required")
	}
	return sensu.CheckStateOK, nil
}

func executeCheck(event *types.Event) (int, error) {
	start, err := cpu.Times(false)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("Error obtaining CPU timings: %v", err)
	}

	duration, err := time.ParseDuration(fmt.Sprintf("%ds", plugin.Interval))
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("Error parsing duration: %v", err)
	}

	time.Sleep(duration)
	end, err := cpu.Times(false)

	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("Error obtaining CPU timings: %v", err)
	}

	//fmt.Println(fieldNames)

	startTotal := 0.0
	endTotal := 0.0

	for _, fieldName := range fieldNames {
		if fieldName != "Total" {
			startTotal += getField(&start[0], fieldName)
			endTotal += getField(&end[0], fieldName)
		}
	}

	diff := endTotal - startTotal

	var fieldValues = map[string]float64{}
	perfData2 := ""

	fieldValues["Total"] = 100 - (((end[0].Idle - start[0].Idle) / diff) * 100)
	for _, fieldName := range fieldNames {
		if fieldName != "Total" {
			fieldValues[fieldName] = ((getField(&end[0], fieldName) - getField(&start[0], fieldName)) / diff) * 100
		}
		perfData2 += fmt.Sprintf("cpu_%s=%.2f ", strings.ToLower(fieldName), fieldValues[fieldName])
	}

	checkState := 0
	if plugin.UsageTypes != "" {

		usageChecks := strings.Split(plugin.UsageTypes, ",")
		//fmt.Println(usageChecks)
		for _, usageCheck := range usageChecks {
			usageCheckParts := strings.Split(usageCheck, ":")
			//fmt.Println(usageCheckParts)

			check := &CpuCheck{
				UsageType: usageCheckParts[0],
			}
			if len(usageCheckParts) > 1 {
				check.Warning, err = strconv.ParseFloat(usageCheckParts[1], 8)
			} else {
				check.Warning = plugin.Warning
			}
			if len(usageCheckParts) > 2 {
				check.Critical, err = strconv.ParseFloat(usageCheckParts[2], 8)
			} else {
				check.Critical = plugin.Critical
			}
			if len(usageCheckParts) > 3 {
				check.Inverted, err = strconv.ParseBool(usageCheckParts[3])
			} else {
				check.Inverted = false
			}
			//fmt.Println(check)
			if err != nil {
				return sensu.CheckStateCritical, fmt.Errorf("Error obtaining CPU timings: %v", err)
			}

			state := checkValue(fieldValues, check)
			if state > checkState {
				checkState = state
			}
		}
	} else {
		check := &CpuCheck{
			UsageType: plugin.UsageType,
			Warning:   plugin.Warning,
			Critical:  plugin.Critical,
			Inverted:  false,
		}
		checkState = checkValue(fieldValues, check)
	}

	fmt.Printf("        | %s\n", perfData2)
	return checkState, nil
}

func checkValue(fieldValues map[string]float64, check *CpuCheck) int {

	checkState := 0
	checkValuePct := fieldValues[check.UsageType]
	checkValueType := strings.ToLower(check.UsageType)

	if checkValuePct > check.Critical {
		fmt.Printf("%s %s Critical: %.2f%% is higher than %.2f%%\n", plugin.PluginConfig.Name, checkValueType, checkValuePct, check.Critical)
		checkState = sensu.CheckStateCritical
	} else if checkValuePct > check.Warning {
		fmt.Printf("%s %s Warning: %.2f%% is higher than %.2f%%\n", plugin.PluginConfig.Name, checkValueType, checkValuePct, check.Warning)
		checkState = sensu.CheckStateWarning
	} else {
		fmt.Printf("%s %s OK: %.2f%% is lower than %.2f%%\n", plugin.PluginConfig.Name, checkValueType, checkValuePct, check.Warning)
	}

	return checkState
}

func getField(v *cpu.TimesStat, field string) float64 {
	value, err := reflections.GetField(v, field)
	if err != nil {
		return 0
	}
	switch i := value.(type) {
	case float64:
		return i
	default:
		return 0
	}
}
