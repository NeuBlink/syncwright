// Package iojson provides JSON I/O utilities for reading from stdin/files and writing to stdout/files
package iojson

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// ReadInput reads JSON from either stdin or a file
func ReadInput(filename string, v interface{}) error {
	var reader io.Reader

	if filename == "" || filename == "-" {
		reader = os.Stdin
	} else {
		file, err := os.Open(filename)
		if err != nil {
			return fmt.Errorf("failed to open input file %s: %w", filename, err)
		}
		defer file.Close()
		reader = file
	}

	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}

	return nil
}

// WriteOutput writes JSON to either stdout or a file
func WriteOutput(filename string, v interface{}) error {
	var writer io.Writer

	if filename == "" || filename == "-" {
		writer = os.Stdout
	} else {
		file, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create output file %s: %w", filename, err)
		}
		defer file.Close()
		writer = file
	}

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(v); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// ReadFile reads and unmarshals JSON from a file
func ReadFile(filename string, v interface{}) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON from %s: %w", filename, err)
	}

	return nil
}

// WriteFile marshals and writes JSON to a file
func WriteFile(filename string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filename, err)
	}

	return nil
}