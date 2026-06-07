package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pterm/pterm"

	"github.com/distiled/orphion/internal/app"
	"github.com/distiled/orphion/internal/config"
)

// sessionConfig holds config overrides for a single download session.
type sessionConfig struct {
	OutputDir   string
	Quality     string
	Concurrency int
	Force       bool
}

// showConfigAndEdit displays the current config, offers to edit, and
// returns session-only overrides. Changes do not persist to disk.
func showConfigAndEdit(cfg *config.Config) (*sessionConfig, error) {
	pterm.DefaultSection.Println("Download Configuration")

	// Show current defaults as a styled table.
	tableData := pterm.TableData{
		{pterm.Cyan("Setting"), pterm.Cyan("Value")},
		{"Output directory", cfg.OutputDir},
		{"Preferred quality", cfg.PreferredQuality},
		{"Concurrency", strconv.Itoa(cfg.Concurrency)},
	}
	if cfg.Provider != "" {
		tableData = append(tableData, []string{"Provider", cfg.Provider})
	}
	tableData = append(tableData, []string{"Overwrite existing", fmt.Sprintf("%v", false)})
	_ = pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()

	// Ask: continue or edit?
	options := []string{"Continue", "Edit"}
	choice, err := pterm.DefaultInteractiveSelect.
		WithOptions(options).
		WithDefaultText("Config:").
		Show()
	if err != nil {
		return nil, fmt.Errorf("config selection: %w", err)
	}

	sc := &sessionConfig{
		OutputDir:   cfg.OutputDir,
		Quality:     cfg.PreferredQuality,
		Concurrency: cfg.Concurrency,
		Force:       false,
	}

	if choice != "Edit" {
		return sc, nil
	}

	// Interactive edit: each prompt shows the current default.
	// Hitting Enter keeps the default value.
	pterm.Info.Println("Edit session config (Enter to keep default)")

	outputDir, err := pterm.DefaultInteractiveTextInput.
		WithDefaultText(fmt.Sprintf("Output dir [%s]: ", cfg.OutputDir)).
		Show()
	if err == nil {
		outputDir = strings.TrimSpace(outputDir)
		if outputDir != "" {
			sc.OutputDir = outputDir
		}
	}

	quality, err := pterm.DefaultInteractiveTextInput.
		WithDefaultText(fmt.Sprintf("Quality [%s]: ", cfg.PreferredQuality)).
		Show()
	if err == nil {
		quality = strings.TrimSpace(quality)
		if quality != "" {
			sc.Quality = quality
		}
	}

	concStr, err := pterm.DefaultInteractiveTextInput.
		WithDefaultText(fmt.Sprintf("Concurrency (1-4) [%d]: ", cfg.Concurrency)).
		Show()
	if err == nil {
		concStr = strings.TrimSpace(concStr)
		if concStr != "" {
			if n, err := strconv.Atoi(concStr); err == nil && n >= 1 && n <= 4 {
				sc.Concurrency = n
			}
		}
	}

	forceChoice, err := pterm.DefaultInteractiveSelect.
		WithOptions([]string{"No", "Yes"}).
		WithDefaultText("Overwrite existing files?").
		Show()
	if err == nil {
		sc.Force = forceChoice == "Yes"
	}

	// Show final session config.
	pterm.Info.Println("Session config:")
	finalTable := pterm.TableData{
		{pterm.Cyan("Setting"), pterm.Cyan("Value")},
		{"Output directory", sc.OutputDir},
		{"Preferred quality", sc.Quality},
		{"Concurrency", strconv.Itoa(sc.Concurrency)},
		{"Overwrite existing", fmt.Sprintf("%v", sc.Force)},
	}
	_ = pterm.DefaultTable.WithHasHeader().WithData(finalTable).Render()

	return sc, nil
}

// applySessionConfig applies the session overrides to the service.
func applySessionConfig(service *app.Service, sc *sessionConfig) {
	service.SetOutputDir(sc.OutputDir)
	service.SetPreferredQuality(sc.Quality)
	service.SetConcurrency(sc.Concurrency)
	service.SetForce(sc.Force)
}
