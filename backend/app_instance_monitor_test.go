package backend

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestIsWindowsBreakpointExit(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "breakpoint hex status", err: errors.New("exit status 0x80000003"), want: true},
		{name: "case insensitive", err: errors.New("EXIT STATUS 0X80000003"), want: true},
		{name: "access violation", err: errors.New("exit status 0xc0000005"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isWindowsBreakpointExit(tt.err); got != tt.want {
				t.Fatalf("isWindowsBreakpointExit(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsWindowsBreakpointExitFromProcessState(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows exit status semantics only")
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestWindowsBreakpointExitHelperProcess")
	cmd.Env = append(os.Environ(), "LIANXIANG_TEST_BREAKPOINT_EXIT=1")
	err := cmd.Run()
	if !isWindowsBreakpointExit(err) {
		t.Fatalf("isWindowsBreakpointExit(%v) = false, want true", err)
	}
}

func TestWindowsBreakpointExitHelperProcess(t *testing.T) {
	if os.Getenv("LIANXIANG_TEST_BREAKPOINT_EXIT") != "1" {
		return
	}
	os.Exit(-2147483645)
}

func TestShouldRetryEarlyBrowserBreakpoint(t *testing.T) {
	err := errors.New("exit status 0x80000003")
	wantWindows := runtime.GOOS == "windows"

	if got := shouldRetryEarlyBrowserBreakpoint(err, 40*time.Second, true); got != wantWindows {
		t.Fatalf("eligible early breakpoint retry = %v, want %v", got, wantWindows)
	}
	if shouldRetryEarlyBrowserBreakpoint(err, 40*time.Second, false) {
		t.Fatal("retry must not repeat after the recovery attempt")
	}
	if shouldRetryEarlyBrowserBreakpoint(err, browserEarlyBreakpointRecoveryWindow+time.Second, true) {
		t.Fatal("late breakpoint exit must not restart a browser session")
	}
	if shouldRetryEarlyBrowserBreakpoint(errors.New("exit status 0xc0000005"), time.Second, true) {
		t.Fatal("non-breakpoint crashes must not be retried")
	}
}
