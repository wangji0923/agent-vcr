package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/analysis"
	"github.com/agent-vcr/agent-vcr/internal/resolver"
	"github.com/agent-vcr/agent-vcr/internal/trace"
	"github.com/spf13/cobra"
)

type replayOptions struct {
	filter string
	step   bool
}

func newReplayCommand(rootOpts *Options) *cobra.Command {
	opts := &replayOptions{}
	cmd := &cobra.Command{
		Use:   "replay <run-id|latest>",
		Short: "Replay a recorded run timeline.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID, err := resolver.Resolve(rootOpts.ProjectDir, args[0])
			if err != nil {
				return err
			}
			replay, err := analysis.LoadReplay(rootOpts.ProjectDir, runID)
			if err != nil {
				return err
			}
			replay.Timeline = analysis.FilterTimeline(replay.Timeline, opts.filter)
			if rootOpts.JSON {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(replay)
			}
			output := renderReplay(replay)
			if opts.step && isInteractiveReader(cmd.InOrStdin()) {
				return printStepped(cmd.OutOrStdout(), cmd.InOrStdin(), output)
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), output)
			return err
		},
	}
	cmd.Flags().StringVar(&opts.filter, "filter", "", "filter timeline: tool, shell, file, raw")
	cmd.Flags().BoolVar(&opts.step, "step", false, "pause after each replay line when stdin is interactive")
	return cmd
}

func renderReplay(replay analysis.Replay) string {
	meta := replay.Metadata
	source := fallbackString(meta.Source, "unknown")
	status := fallbackString(meta.Status, trace.RunStatusUnknown)
	gitSHA := fallbackString(meta.GitSHA, "-")

	var out strings.Builder
	fmt.Fprintf(&out, "Run: %s\n", replay.RunID)
	fmt.Fprintf(&out, "Source: %s\n", source)
	fmt.Fprintf(&out, "Status: %s\n", status)
	fmt.Fprintf(&out, "Git: %s\n\n", gitSHA)

	start := timelineStart(meta, replay.Timeline)
	for _, item := range replay.Timeline {
		fmt.Fprintf(&out, "%s %-14s %s\n", relativeTime(start, item.Time), item.Type, timelineText(item))
	}
	return out.String()
}

func timelineStart(meta trace.Metadata, items []analysis.TimelineItem) time.Time {
	if !meta.StartedAt.IsZero() {
		return meta.StartedAt
	}
	for _, item := range items {
		if !item.Time.IsZero() {
			return item.Time
		}
	}
	return time.Time{}
}

func relativeTime(start time.Time, value time.Time) string {
	if start.IsZero() || value.IsZero() {
		return "--:--"
	}
	if value.Before(start) {
		value = start
	}
	duration := value.Sub(start).Round(time.Second)
	hours := int(duration / time.Hour)
	duration -= time.Duration(hours) * time.Hour
	minutes := int(duration / time.Minute)
	duration -= time.Duration(minutes) * time.Minute
	seconds := int(duration / time.Second)
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func timelineText(item analysis.TimelineItem) string {
	text := item.Title
	if item.Detail != "" {
		text = strings.TrimSpace(text + " -> " + item.Detail)
	}
	if len(item.ChangedFiles) > 0 {
		files := strings.Join(item.ChangedFiles, ", ")
		if item.Detail == "" {
			text = strings.TrimSpace(text + " -> changed " + fmt.Sprintf("%d", len(item.ChangedFiles)) + " files")
		}
		text = strings.TrimSpace(text + " [" + files + "]")
	}
	if artifacts := artifactSummary(item); artifacts != "" {
		text = strings.TrimSpace(text + " {" + artifacts + "}")
	}
	if text == "" {
		return item.Type
	}
	return text
}

func artifactSummary(item analysis.TimelineItem) string {
	if item.Type == "raw" {
		return ""
	}
	var parts []string
	seen := map[string]bool{}
	for _, artifact := range item.Artifacts {
		if artifact.Path == "" || seen[artifact.Path] {
			continue
		}
		seen[artifact.Path] = true
		value := artifact.Path
		if artifact.SizeBytes > 0 {
			value += fmt.Sprintf(" (%d bytes)", artifact.SizeBytes)
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, ", ")
}

func printStepped(out io.Writer, in io.Reader, output string) error {
	reader := bufio.NewReader(in)
	for _, line := range strings.SplitAfter(output, "\n") {
		if line == "" {
			continue
		}
		if _, err := fmt.Fprint(out, line); err != nil {
			return err
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		next, err := reader.ReadString('\n')
		if err != nil {
			return nil
		}
		if strings.EqualFold(strings.TrimSpace(next), "q") {
			return nil
		}
	}
	return nil
}

func isInteractiveReader(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
