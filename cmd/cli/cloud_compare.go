package main

import (
	"github.com/spf13/cobra"
)

func cloudCompareCmd() *cobra.Command {
	var runA, runB string
	var outputFiles []string
	var threshold float64
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare metrics between two runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newCloudClient()
			if err != nil {
				return err
			}
			return runCompare(c, runA, runB, threshold, outputFiles)
		},
	}
	cmd.Flags().StringVar(&runA, "run-a", "", "first run ID (baseline)")
	cmd.Flags().StringVar(&runB, "run-b", "", "second run ID")
	cmd.Flags().StringArrayVarP(&outputFiles, "output", "o", nil, "output file (format from extension: .md .json .xml); repeatable")
	cmd.Flags().Float64Var(&threshold, "threshold", 0, "custom threshold percentage (0 = server default)")
	cmd.MarkFlagRequired("run-a")
	cmd.MarkFlagRequired("run-b")
	return cmd
}
