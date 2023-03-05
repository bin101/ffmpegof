package main

import (
	"fmt"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"github.com/aleksasiriski/rffmpeg-go/processor"
	"github.com/rs/zerolog/log"
)

type Add struct {
	Name   string `help:"Name of the server." short:"n" optional:""`
	Weight int    `help:"Weight of the server." short:"w" default:"1" optional:""`
	Host   string `arg:"" name:"host" help:"Hostname or IP." required:""`
}

type Rm struct {
	Name string `arg:"" name:"host" help:"Name of the server." required:""`
}

type Cli struct {
	Add    Add      `cmd:"" help:"Add host."`
	Rm     Rm       `cmd:"" help:"Remove host."`
	Status struct{} `cmd:"" help:"Status of all hosts."`
}

func addHost(proc *processor.Processor, info Add) error {
	if info.Name == "" {
		info.Name = info.Host
	}

	err := proc.AddHost(processor.Host{
		Servername: info.Name,
		Hostname:   info.Host,
		Weight:     info.Weight,
		Created:    time.Now(),
	})

	return err
}

func removeHost(proc *processor.Processor, info Rm) error {
	err := proc.RemoveHost(processor.Host{
		Servername: info.Name,
	})

	return err
}

type StatusMapping struct {
	Id           string
	Servername   string
	Hostname     string
	Weight       string
	CurrentState string
	Commands     []processor.Process
}

func printStatus(statusMappings []StatusMapping) {
	servernameLen := 11
	hostnameLen := 9
	idLen := 3
	weightLen := 7
	stateLen := 6
	for _, statusMapping := range statusMappings {
		if len(statusMapping.Servername)+1 > servernameLen {
			servernameLen = len(statusMapping.Servername) + 1
		}
		if len(statusMapping.Hostname)+1 > hostnameLen {
			hostnameLen = len(statusMapping.Hostname) + 1
		}
		if len(statusMapping.Id)+1 > idLen {
			idLen = len(statusMapping.Id) + 1
		}
		if len(statusMapping.Weight)+1 > weightLen {
			weightLen = len(statusMapping.Weight) + 1
		}
		if len(statusMapping.CurrentState)+1 > stateLen {
			stateLen = len(statusMapping.CurrentState) + 1
		}
	}

	output := make([]string, 0)
	output = append(output, fmt.Sprintf("%-s%-*s %-*s %-*s %-*s %-*s %-s%-s",
		"\033[1m",
		servernameLen,
		"Servername",
		hostnameLen,
		"Hostname",
		idLen,
		"ID",
		weightLen,
		"Weight",
		stateLen,
		"State",
		"Active Commands",
		"\033[0m",
	))

	for _, statusMapping := range statusMappings {
		firstCommand := "N/A"
		if len(statusMapping.Commands) > 0 {
			firstCommand = fmt.Sprintf("PID %-d: %-s", statusMapping.Commands[0].ProcessId, statusMapping.Commands[0].Cmd)
		}

		mappingOutput := make([]string, 0)
		mappingOutput = append(mappingOutput, fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s %-s",
			servernameLen,
			statusMapping.Servername,
			hostnameLen,
			statusMapping.Hostname,
			idLen,
			statusMapping.Id,
			weightLen,
			statusMapping.Weight,
			stateLen,
			statusMapping.CurrentState,
			firstCommand,
		))

		for index, command := range statusMapping.Commands {
			if index != 0 {
				formattedCommand := fmt.Sprintf("PID %d: %s", command.ProcessId, command.Cmd)
				mappingOutput = append(mappingOutput, fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s %-s",
					servernameLen,
					"",
					hostnameLen,
					"",
					idLen,
					"",
					weightLen,
					"",
					stateLen,
					"",
					formattedCommand,
				))
			}
		}

		output = append(output, mappingOutput...)
	}

	for _, line := range output {
		fmt.Printf("%s\n", line)
	}
}

func status(proc *processor.Processor) error {
	hosts, err := proc.GetHosts()
	if err != nil {
		return err
	}

	// Determine if there are any fallback processes running
	fallbackProcesses, err := proc.GetProcessesFromHost(processor.Host{
		Id: 0,
	})
	if err != nil {
		return err
	}

	// Generate a mapping dictionary of hosts and processes
	statusMappings := make([]StatusMapping, 0)

	if len(fallbackProcesses) > 0 {
		statusMappings = append(statusMappings, StatusMapping{
			Id:           "0",
			Servername:   "localhost (fallback)",
			Hostname:     "localhost",
			Weight:       "0",
			CurrentState: "fallback",
			Commands:     fallbackProcesses,
		})
	}

	for _, host := range hosts {
		// Get the latest state
		states, err := proc.GetStatesFromHost(host)
		if err != nil {
			return err
		}

		currentState := ""
		if len(states) == 0 {
			currentState = "idle"
		} else {
			currentState = states[0].State
		}

		// Get processes from host
		processes, err := proc.GetProcessesFromHost(host)
		if err != nil {
			return err
		}

		// Create the mappings entry
		statusMappings = append(statusMappings, StatusMapping{
			Id:           fmt.Sprintf("%d", host.Id),
			Servername:   host.Servername,
			Hostname:     host.Hostname,
			Weight:       fmt.Sprintf("%d", host.Weight),
			CurrentState: currentState,
			Commands:     processes,
		})
	}

	log.Info().
		Msg("Outputting status of hosts")
	printStatus(statusMappings)

	return err
}

func runControl(proc *processor.Processor) {
	// parse cli
	cli := Cli{}

	ctx := kong.Parse(&cli,
		kong.Name("rffmpeg"),
		kong.Description("Remote ffmpeg"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Summary: true,
			Compact: true,
		}),
	)

	if err := ctx.Validate(); err != nil {
		log.Error().
			Err(err).
			Msg("Failed parsing cli:")
		os.Exit(1)
	}

	// functions based on arguments
	switch ctx.Command() {
	case "add <host>":
		{
			err := addHost(proc, cli.Add)
			if err != nil {
				log.Error().
					Err(err).
					Msg("Failed adding host:")
			} else {
				log.Info().
					Msg("Succesfully added host")
			}
		}
	case "rm <host>":
		{
			err := removeHost(proc, cli.Rm)
			if err != nil {
				log.Error().
					Err(err).
					Msg("Failed removing host:")
			} else {
				log.Info().
					Msg("Succesfully removed host")
			}
		}
	case "status":
		{
			err := status(proc)
			if err != nil {
				log.Error().
					Err(err).
					Msg("Failed reading status:")
			}
		}
	default:
		{
			log.Fatal().
				Err(fmt.Errorf("%s", ctx.Command())).
				Msg("Invalid command:")
		}
	}
}
