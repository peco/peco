package peco

import (
	"os"
	"strconv"
	"strings"
)

/*
	Find the index of the column that contains PID, if it exists.
	If not, this is probably not a ps output.
*/
func findPIDIndex(line string) (int, bool) {

	for idx, chunk := range splitAndTrim(line) {
		if chunk == "PID" {
			return idx, true
		}
	}
	return 0, false
}

/*
	If the pid in the given column is valid, try to kill it
*/
func killPID(line string, idx int) bool {
	cols := splitAndTrim(line)
	pid, err := strconv.Atoi(cols[idx])

	if err != nil {
		return false
	}

	if proc, err := os.FindProcess(pid); err == nil {
		err = proc.Kill()
		return err == nil
	}
	return false
}

func splitAndTrim(in string) []string {
	out := []string{}
	for _, col := range strings.Split(in, " ") {
		if col != "" {
			out = append(out, col)
		}
	}
	return out
}
