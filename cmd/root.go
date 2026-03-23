package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gorm"
)

var (
	Version   = "dev"
	BuildDate = "unknown"
)

type config struct {
	host   string
	port   int
	user   string
	pass   string
	dbName string
	out    string
	merge  bool
}

var cfg = config{}
var envFile string

var rootCmd = &cobra.Command{
	Use:   "sql2go",
	Short: "Generate Go models from MySQL tables",
	Long:  "sql2go generates Go models from all tables in a MySQL database using gorm.io/gen.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
	SilenceUsage: true,
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate Go models from a MySQL database",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load .env file (ignore error if file not found, unless explicitly specified)
		if envFile != "" {
			if err := godotenv.Load(envFile); err != nil && cmd.Flags().Changed("env") {
				return fmt.Errorf("failed to load env file %q: %w", envFile, err)
			}
		}

		// Apply env vars for flags not explicitly set by the user
		if !cmd.Flags().Changed("host") {
			if v := os.Getenv("SQL2GO_HOST"); v != "" {
				cfg.host = v
			}
		}
		if !cmd.Flags().Changed("port") {
			if v := os.Getenv("SQL2GO_PORT"); v != "" {
				if p, err := strconv.Atoi(v); err == nil {
					cfg.port = p
				}
			}
		}
		if !cmd.Flags().Changed("user") {
			if v := os.Getenv("SQL2GO_USER"); v != "" {
				cfg.user = v
			}
		}
		if !cmd.Flags().Changed("pass") {
			if v := os.Getenv("SQL2GO_PASS"); v != "" {
				cfg.pass = v
			}
		}
		if !cmd.Flags().Changed("db") {
			if v := os.Getenv("SQL2GO_DB"); v != "" {
				cfg.dbName = v
			}
		}
		if !cmd.Flags().Changed("out") {
			if v := os.Getenv("SQL2GO_OUT"); v != "" {
				cfg.out = v
			}
		}
		if !cmd.Flags().Changed("merge") {
			if v := os.Getenv("SQL2GO_MERGE"); v != "" {
				cfg.merge = v == "true" || v == "1"
			}
		}

		if cfg.dbName == "" {
			return fmt.Errorf("required flag \"db\" not set and SQL2GO_DB env var is empty")
		}

		return runGenerator(cfg)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "sql2go version %s\n", Version)
		fmt.Fprintf(cmd.OutOrStdout(), "build date: %s\n", BuildDate)
	},
}

func init() {
	generateCmd.Flags().SortFlags = false
	generateCmd.Flags().StringVar(&envFile, "env", ".env", "Path to .env file (loaded automatically if it exists)")
	generateCmd.Flags().StringVar(&cfg.host, "host", "127.0.0.1", "MySQL host (env: SQL2GO_HOST)")
	generateCmd.Flags().IntVar(&cfg.port, "port", 3306, "MySQL port (env: SQL2GO_PORT)")
	generateCmd.Flags().StringVar(&cfg.user, "user", "root", "MySQL user (env: SQL2GO_USER)")
	generateCmd.Flags().StringVar(&cfg.pass, "pass", "", "MySQL password (env: SQL2GO_PASS)")
	generateCmd.Flags().StringVar(&cfg.dbName, "db", "", "Database name (env: SQL2GO_DB)")
	generateCmd.Flags().StringVar(&cfg.out, "out", "./models", "Output directory (env: SQL2GO_OUT)")
	generateCmd.Flags().BoolVar(&cfg.merge, "merge", false, "Merge generated models into a single file (env: SQL2GO_MERGE)")

	rootCmd.AddCommand(generateCmd, versionCmd)

	// Keep the standard library flag package from interfering with Cobra when imported by dependencies.
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
}

func Execute() error {
	return rootCmd.Execute()
}

func runGenerator(cfg config) error {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.user, cfg.pass, cfg.host, cfg.port, cfg.dbName,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	fmt.Println("Connected successfully")

	g := gen.NewGenerator(gen.Config{
		OutPath:           cfg.out + "/query",
		ModelPkgPath:      cfg.out,
		FieldNullable:     true,
		FieldWithIndexTag: true,
		FieldWithTypeTag:  true,
	})

	g.UseDB(db)
	g.ApplyBasic(g.GenerateAllTable()...)
	g.Execute()

	if err := os.RemoveAll(cfg.out + "/query"); err != nil {
		fmt.Println("Failed to remove query folder:", err)
	}

	if cfg.merge {
		if err := mergeGeneratedModels(cfg.out, cfg.dbName); err != nil {
			fmt.Println("Failed to merge generated files:", err)
		}
	}

	fmt.Println("Models generated in:", cfg.out)
	return nil
}

func mergeGeneratedModels(outDir string, dbName string) error {
	var allCode strings.Builder
	allImports := make(map[string]bool)

	err := filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".gen.go") {
			if info.Name() == "gen.go" {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			lines := strings.Split(string(content), "\n")
			inImport := false

			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "package ") || strings.HasPrefix(trimmed, "// Code generated") {
					continue
				}
				if trimmed == "import (" {
					inImport = true
					continue
				}
				if inImport {
					if trimmed == ")" {
						inImport = false
					} else if trimmed != "" {
						allImports[trimmed] = true
					}
					continue
				}
				if strings.HasPrefix(trimmed, "import ") {
					allImports[strings.TrimPrefix(trimmed, "import ")] = true
					continue
				}
				allCode.WriteString(line + "\n")
			}

			os.Remove(path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if allCode.Len() == 0 {
		return nil
	}

	var finalFile strings.Builder
	pkgName := filepath.Base(outDir)
	finalFile.WriteString(fmt.Sprintf("// Code generated by sql2go. DO NOT EDIT.\n\npackage %s\n\n", pkgName))

	if len(allImports) > 0 {
		finalFile.WriteString("import (\n")
		for imp := range allImports {
			finalFile.WriteString("\t" + imp + "\n")
		}
		finalFile.WriteString(")\n")
	}

	finalFile.WriteString(allCode.String())

	finalPath := filepath.Join(outDir, dbName+".go")
	return os.WriteFile(finalPath, []byte(finalFile.String()), 0o644)
}
