// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows

package main

import (
	"github.com/huuzkee-foundation/winsvc/debug"
	"github.com/huuzkee-foundation/winsvc/eventlog"
	"github.com/huuzkee-foundation/winsvc/svc"
	"fmt"
	"time"
    	"os"
    	//"strings"
    	"io"
    	//"bytes"
	"os/exec"
	"log"
)

var elog debug.Log

type myservice struct{}


func launcher(cmdstid *io.PipeReader,  outfile *os.File, winlog *eventlog.Log, service string) {


   	outfile.WriteString( "\r\nSTARTING CMD LAUNCHER for " )
   	outfile.WriteString( service )
     	outfile.WriteString( " \r\n  \r\n" )
     	
        cmdName := "cmd"
	cmdArgs := []string{" "}
	cmd := exec.Command( cmdName, cmdArgs... )
	cmd.Stdin = cmdstid
    	cmd.Stdout = outfile 
	errCmd := cmd.Start()
		if errCmd != nil {
			errWl := winlog.Info(1, "cmd.Start() FAILED")
			if errWl != nil {
				log.Printf("Info failed: %s", errWl)
			}
		}
	errWl := winlog.Info(1, "CMD LAUNCHER STARTED")
		if errWl != nil {
			log.Printf("Info failed: %s", errWl)
		}
	cmd.Wait()
}

func (m *myservice) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
	changes <- svc.Status{State: svc.StartPending}
	fasttick := time.Tick(500 * time.Millisecond)
	slowtick := time.Tick(2 * time.Second)
	tick := fasttick
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	
	MIXER_SERVICE			:= "MIXER-SERVICE"
	MIXER_ROOT 			:= "C:\\Program Files\\MixWorks\\MixerWinSvc\\"
	MIXER_LOG			:= MIXER_ROOT + MIXER_SERVICE + ".LOG"
	MIXER_START			:= MIXER_ROOT + "startmixer.bat"
	MIXER_SEMAPHORE_RUNNING  	:= MIXER_ROOT + MIXER_SERVICE + ".IS.RUNNING"
	MIXER_SEMAPHORE_STOPPED  	:= MIXER_ROOT + MIXER_SERVICE + ".IS.STOPPED"
	
	const supports = eventlog.Error | eventlog.Warning | eventlog.Info
	err := eventlog.InstallAsEventCreate(MIXER_SERVICE, supports)
		if err != nil {
			log.Printf("Event Log Install failed: %s", err)
		}
	winlog, err := eventlog.Open(MIXER_SERVICE)
		if err != nil {
			log.Printf("Event Log Open failed: %s", err)
		}
	defer 	winlog.Close()
	err = 	winlog.Info(1, "STARTING" )
		if err != nil {
			log.Printf("Event Log Info failed: %s", err)
		}

	// open the out file for writing
	cmdstdout, err := os.Create( MIXER_LOG )
		if err != nil {
			err = winlog.Info(1, "FAILED outfile")
			if err != nil {
				log.Printf("Event Log Info failed: %s", err)
			}
		}
   	defer cmdstdout.Close()
   	   	
	cmdstdin,cmdcon  := io.Pipe()
	go launcher( cmdstdin, 	cmdstdout, winlog, MIXER_SERVICE )
	  	
	cmdstdout.WriteString( "\r\n" )
   	cmdstdout.WriteString( MIXER_SERVICE + " is Running " )	
	cmdstdout.WriteString( "\r\n" )
	
   	cmdcon.Write( []byte("\r\n") )
      	cmdcon.Write( []byte("cd ") )
      	cmdcon.Write( []byte( MIXER_ROOT ) )
     	cmdcon.Write( []byte("\r\n") )
      	cmdcon.Write( []byte("del ") )
      	cmdcon.Write( []byte( MIXER_SEMAPHORE_STOPPED ) )
     	cmdcon.Write( []byte("\r\n") )
     	
	semaphore, err := os.Create( MIXER_SEMAPHORE_RUNNING )
	semaphore.WriteString( "\r\n" )
	semaphore.WriteString( MIXER_SERVICE + " is Running " )	
	semaphore.WriteString( "\r\n" )
	
   	cmdcon.Write( []byte( MIXER_START ) )
	cmdcon.Write( []byte(" \"run -Dhttp.port=80  -Dhttps.port=443\"") )
   	cmdcon.Write( []byte("\r\n") )
   	   	
loop:
	for {
		select {
		case <-tick:
			//outfile.flush()
			beep()
			
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				// testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				semaphore.Close()
			        os.Rename( MIXER_SEMAPHORE_RUNNING, MIXER_SEMAPHORE_STOPPED )
				err = winlog.Info(1, "SERVICE CLOSED")
				break loop
			case svc.Pause:
				changes <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}
				tick = slowtick
			case svc.Continue:
				changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
				tick = fasttick
			default:
				elog.Error(1, fmt.Sprintf("unexpected control request #%d", c))
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

func runService(name string, isDebug bool) {
	var err error
	if isDebug {
		elog = debug.New(name)
	} else {
		elog, err = eventlog.Open(name)
		if err != nil {
			return
		}
	}
	defer elog.Close()

	elog.Info(1, fmt.Sprintf("starting %s service", name))
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &myservice{})
	if err != nil {
		elog.Error(1, fmt.Sprintf("%s service failed: %v", name, err))
		return
	}
	elog.Info(1, fmt.Sprintf("%s service stopped", name))
}
