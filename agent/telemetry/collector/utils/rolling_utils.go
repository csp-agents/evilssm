// Copyright 2025 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.
package utils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const seelogConfigStringFormat = `
<seelog type="adaptive" mininterval="2000000" maxinterval="100000000" critmsgcount="500" minlevel="trace">
    <outputs formatid="common">
	<rollingfile type="size" filename="%v" maxsize="%v" maxrolls="%v"/>
    </outputs>
    <formats>
        <format id="common" format="%%Msg%%n"/>
    </formats>
</seelog>`

func GetLoggerConfig(defaultLogDir, logFile string, maxRolls int, maxFileSize int64) []byte {
	logFilePath := filepath.Join(defaultLogDir, logFile)
	logConfig := fmt.Sprintf(seelogConfigStringFormat, logFilePath, maxFileSize, maxRolls)
	return []byte(logConfig)
}

// getReverseSortedLogFiles retrieves and sorts file names from the current directory.
// It filters valid roll files based on the collector's naming pattern and returns
// their full file paths in descending order. This makes sure that the first file has the latest logs
// and so on.
func getReverseSortedLogFiles(dirPath, expectedPrefix, expectedRollDelimiter string) ([]string, error) {
	files, err := ListFiles(dirPath, nil)
	if err != nil {
		return nil, err
	}
	var validRollNumbers []int
	for _, file := range files {
		if matchesRollPattern(file, expectedPrefix, expectedRollDelimiter) {
			fileNumber, err := getFileRollSuffix(expectedPrefix, expectedRollDelimiter, file)
			if err == nil {
				validRollNumbers = append(validRollNumbers, fileNumber)
			}
		}
	}
	sort.Ints(validRollNumbers)
	slices.Reverse(validRollNumbers)

	// Check if the latest (which doesn't have the number suffix) file is present
	// include it if it is present
	latestFileExists := slices.Contains(files, expectedPrefix)

	var resultSize, offset int
	if latestFileExists {
		offset = 1
	} else {
		offset = 0
	}

	resultSize = len(validRollNumbers) + offset

	validSortedFiles := make([]string, resultSize)

	if latestFileExists {
		validSortedFiles[0] = filepath.Join(dirPath, expectedPrefix)
	}

	for i, v := range validRollNumbers {
		validSortedFiles[i+offset] = filepath.Join(dirPath, createFullFileName(expectedPrefix, expectedRollDelimiter, v))
	}
	return validSortedFiles, nil
}

// getFileRollSuffix returns the file roll suffix
// For example, for log.4, it returns 4
func getFileRollSuffix(expectedPrefix, expectedRollDelimiter, fileName string) (int, error) {
	return strconv.Atoi(fileName[len(expectedPrefix+expectedRollDelimiter):])
}

// matchesRollPattern returns true if the given file name matches the pattern expected from roll files
func matchesRollPattern(file, expectedPrefix, expectedRollDelimiter string) bool {
	rname := expectedPrefix + expectedRollDelimiter
	return strings.HasPrefix(file, rname)
}

func createFullFileName(originalName, rollDelimiter string, rollNumber int) string {
	return originalName + rollDelimiter + strconv.Itoa(rollNumber)
}

// getSubdirNames returns a list of directories found in
// the given one with dirPath.
func getSubdirNames(dirPath string) ([]string, error) {
	fi, err := os.Stat(dirPath)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%v is not a directory", dirPath)
	}
	dd, err := os.Open(dirPath)
	// Cannot open file.
	if err != nil {
		if dd != nil {
			dd.Close()
		}
		return nil, err
	}
	defer dd.Close()
	allEntities, err := dd.Readdir(-1)
	if err != nil {
		return nil, err
	}
	subDirs := []string{}
	for _, entity := range allEntities {
		if entity.IsDir() {
			subDirs = append(subDirs, entity.Name())
		}
	}
	return subDirs, nil
}

// ListFiles return full paths of the files located in the directory.
func ListFiles(dirPath string, filterFun func(filePath string) bool) ([]string, error) {
	dfi, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}
	defer dfi.Close()

	result := []string{}

	for {
		// Read in reasonable chunks to prevent loading too much data into memory
		items, err := dfi.Readdir(50)

		if err != nil {
			if err == io.EOF {
				break
			} else {
				return nil, err
			}
		}

		for _, item := range items {
			if item.Mode()&os.ModeType == 0 { // check that it's a regular file (not a symlink etc)
				fp := item.Name()
				if filterFun != nil && !filterFun(fp) {
					continue
				}
				result = append(result, fp)
			}
		}
	}
	return result, nil
}

// ReadAndDeleteRollingLogs is the common logic used fetching written records for both log and metric collectors.
// It reads rolling files from all namespaces, while staying within the specified limit. It deletes the lines which were read.
// returns a namespace -> lines map. returns EOF as error if all the logs were read from all the namespaces
func ReadAndDeleteRollingLogs(log log.BasicT, baseDir, fileNamePrefix string, limit int) (map[string][]string, error) {
	resultMap := make(map[string][]string)

	namespaces, err := getSubdirNames(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			// we probably have not received any telemetry yet. Ignore
			log.Debugf("Directory %v not found, ignoring", baseDir)
			return resultMap, io.EOF
		}

		return nil, err
	}

	sort.Strings(namespaces) // sort for predictability in testing

	namespaceDirs := []string{}
	for _, ns := range namespaces {
		namespaceDirs = append(namespaceDirs, filepath.Join(baseDir, ns))
	}

	remainingLimit := limit

	// TODO: prevent one namespace from starving others

	allFilesReadFully := true

	for i, ns := range namespaces {
		if remainingLimit <= 0 {
			allFilesReadFully = false
			break
		}

		nsDir := namespaceDirs[i]

		var allLines []string
		allLines, readErr := readDirAndTruncate(nsDir, fileNamePrefix, remainingLimit, log)

		if readErr != nil {
			if readErr != io.EOF {
				return nil, readErr
			}
		} else {
			// we didn't read the directory fully
			allFilesReadFully = false
		}

		if len(allLines) > 0 {
			// add to result
			resultMap[ns] = allLines

			remainingLimit = max(0, remainingLimit-len(allLines))
		}
	}

	if allFilesReadFully {
		err = io.EOF
	} else {
		err = nil
	}

	return resultMap, err
}

// readDirAndTruncate reads at most maxLines lines from all the files matching fileNamePrefix from the specified directory
// returns EOF if all matching files were completely read. Returns some other error if there was an actual error
func readDirAndTruncate(dir, fileNamePrefix string, maxLines int, log log.BasicT) (allLines []string, err error) {
	files, err := getReverseSortedLogFiles(dir, fileNamePrefix, ".")
	if err != nil {
		return nil, err
	}

	allLines = make([]string, 0, maxLines)
	allFilesReadFully := true

	if len(files) == 0 {
		return allLines, io.EOF
	}

	for _, file := range files {
		if maxLines <= 0 {
			allFilesReadFully = false
			break
		}

		// read and truncate file
		lines, readErr := readLinesAndTruncate(file, maxLines)

		if readErr != nil {
			if os.IsNotExist(readErr) {
				log.Debugf("File %v not found, ignoring", file)
				continue
			} else if os.IsPermission(readErr) {
				log.Debugf("File %v not accessible, ignoring", file)
				continue
			} else if readErr != io.EOF {
				return nil, readErr
			} else if filepath.Base(file) != fileNamePrefix {
				// the file is completely empty, delete it
				// But do not delete the file the logger is currently writing to
				// the logger will throw an error if it cannot find the file
				os.Remove(file) // ignore error on deletion
			}
		} else {
			// not an EOF so we didn't read the file fully
			allFilesReadFully = false
		}

		allLines = append(allLines, lines...)

		maxLines = max(0, maxLines-len(lines))
	}

	if allFilesReadFully {
		err = io.EOF
	} else {
		err = nil
	}
	return allLines, err
}

// readLinesAndTruncate reads at most maxLines lines of a file from the tail and truncates the file upto that point
// returns the read lines and error. The error is EOF if the file was fully read
func readLinesAndTruncate(filePath string, maxLines int) ([]string, error) {
	file, err := os.Open(filePath)

	if err != nil {
		return nil, err
	}
	defer file.Close()

	// use a rotating buffer so we can read in one pass
	result := make([]string, 0, maxLines)
	resultWriteIndex := 0

	var bytesInResult int64 = 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		bytes := scanner.Bytes()

		if len(result) == maxLines {
			bytesInResult += int64(len(bytes)) - int64(len(result[resultWriteIndex]))

			result[resultWriteIndex] = string(bytes)
		} else {
			result = append(result, string(bytes))

			bytesInResult += int64(len(bytes) + 1) // +1 for newline
		}

		resultWriteIndex = (resultWriteIndex + 1) % maxLines
	}

	if scannerErr := scanner.Err(); scannerErr != nil {
		return nil, scannerErr
	}

	st, err := file.Stat()
	if err != nil {
		return nil, err
	}

	// truncate the file
	err = os.Truncate(filePath, st.Size()-bytesInResult)
	if err != nil {
		return nil, err
	}

	st, err = file.Stat()
	if err != nil {
		return nil, err
	}

	if st.Size() == 0 {
		err = io.EOF
	}

	// rotate the results so they're in the correct order
	// the latest log will be at the start. this helps in testing
	slices.Reverse(result[0:resultWriteIndex])
	slices.Reverse(result[resultWriteIndex:])

	return result, err
}

// DeleteDirectoryIfAllFilesEmpty deletes all the empty files from the given directory.
// It also deletes the directory if the directory is empty after all the files are deleted.
// It does not work recursively.
func DeleteDirectoryIfAllFilesEmpty(dirPath string) error {
	// Open the directory
	dir, err := os.Open(dirPath)
	if err != nil {
		return fmt.Errorf("failed to open directory: %v", err)
	}
	defer dir.Close()

	// Read directory entries
	entries, err := dir.Readdir(-1)
	if err != nil {
		return fmt.Errorf("failed to read directory: %v", err)
	}

	// If directory is empty, delete it
	if len(entries) == 0 {
		return os.Remove(dirPath)
	}

	// Check all files in the directory
	allFilesEmpty := true
	for _, entry := range entries {
		// Skip subdirectories
		if entry.IsDir() || entry.Size() > 0 {
			allFilesEmpty = false
		} else {
			// remove the empty file
			if err := os.Remove(filepath.Join(dirPath, entry.Name())); err != nil {
				return fmt.Errorf("failed to remove file: %v", err)
			}
		}
	}

	// If all files are empty (0 bytes), delete the directory
	if allFilesEmpty {
		return os.Remove(dirPath)
	}
	return nil
}

func UnmarshalList[N interface{}](lines []string, log log.T) []N {
	logs := make([]N, 0, len(lines))

	for _, line := range lines {
		var entry N
		unmarshalErr := json.Unmarshal([]byte(line), &entry)
		if unmarshalErr != nil {
			log.Debugf("Malformed telemetry line : %v", line)
			continue // ignore malformed lines
		}
		logs = append(logs, entry)
	}

	return logs
}
