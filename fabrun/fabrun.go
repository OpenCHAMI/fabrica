// SPDX-License-Identifier: MIT

// Command fabrun starts and stops a Fabrica service in Docker using Compose.
//
// It detects the service's storage backend (file storage, or ent storage with
// a sqlite or postgres database driver), builds a container image from an
// embedded, generic Dockerfile, then runs the service with `docker compose`
// using an embedded, generic compose file (one each for file, sqlite and
// postgres).
// For postgres-backed services the compose stack also includes a postgres
// container.
//
// The compose files are parameterised through environment variables that
// fabrun supplies (and the user may override), notably FABRUN_IMAGE and
// FABRUN_POSTGRES_IMAGE to select images and FABRUN_NAME to set container
// names.
//
// Usage:
//
//	fabrun start [--service PATH | --bin PATH] [--name NAME] [--port PORT] [--db file|sqlite|postgres]
//	fabrun stop  [--service PATH | --bin PATH] [--name NAME] [--keep-volumes]
//	fabrun sql   [--name NAME] [--db sqlite|postgres] "QUERY"
//
// When no binary path is given, fabrun reads BINARY_NAME from the Makefile in
// the current directory and uses bin/$BINARY_NAME. When no name is given, the
// executable's base name is used as the name.
package main

import (
	"bufio"
	"embed"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed Dockerfile.file Dockerfile.sqlite Dockerfile.postgres compose.file.yml compose.sqlite.yml compose.postgres.yml
var assets embed.FS

const (
	servicePort   = "8080" // container-internal and default host service port
	defaultPgPort = "5433" // default host port for the postgres container
)

// dbKind identifies the detected (or chosen) storage backend.
type dbKind string

const (
	dbFile     dbKind = "file"
	dbSQLite   dbKind = "sqlite"
	dbPostgres dbKind = "postgres"
)

// options holds the parsed command-line options for a start/stop/sql invocation.
type options struct {
	serviceDir  string   // service source directory (--service)
	binPath     string   // explicit path to the service binary (--bin)
	name        string   // container / service name (--name)
	port        string   // host port to publish (--port)
	pgPort      string   // host port for postgres (--pg-port)
	autoPorts   bool     // pick free ports if preferred ones are taken (--auto-ports)
	dbForce     dbKind   // forced db backend (--db), empty means auto-detect
	keepVolumes bool     // keep the data volume on stop (--keep-volumes)
	verbose     bool     // print diagnostics (-v/--verbose)
	rest        []string // positional arguments (e.g. the sql query)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cmd := os.Args[1]
	switch cmd {
	case "start":
		opts, err := parseFlags(os.Args[2:])
		if err != nil {
			fatal(err)
		}
		if len(opts.rest) > 0 {
			fatal(fmt.Errorf("unexpected argument %q", opts.rest[0]))
		}
		if err := runStart(opts); err != nil {
			fatal(err)
		}
	case "stop":
		opts, err := parseFlags(os.Args[2:])
		if err != nil {
			fatal(err)
		}
		if len(opts.rest) > 0 {
			fatal(fmt.Errorf("unexpected argument %q", opts.rest[0]))
		}
		if err := runStop(opts); err != nil {
			fatal(err)
		}
	case "sql":
		opts, err := parseFlags(os.Args[2:])
		if err != nil {
			fatal(err)
		}
		if len(opts.rest) == 0 {
			fatal(fmt.Errorf("sql requires a query argument, e.g. fabrun sql \"select * from resources;\""))
		}
		if err := runSQL(opts, strings.Join(opts.rest, " ")); err != nil {
			fatal(err)
		}
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "fabrun: unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `fabrun - start and stop a Fabrica service in Docker

Usage:
  fabrun start [flags]
  fabrun stop  [flags]
  fabrun sql   [flags] "QUERY"

Flags:
  --service PATH   Path to the service source directory (resolves bin/$BINARY_NAME
                   from PATH/Makefile)
  --bin PATH       Path to the service binary executable
  --name NAME      Container/service name (default: executable base name)
  --port PORT      Host port to publish (start only, default: 8080)
  --pg-port PORT   Host port for postgres (start only, default: 5433)
  --auto-ports     Prefer 8080/5433 but pick other free ports if they are taken
                   (start only; without this, in-use ports cause failure)
  --db KIND        Force storage backend: file, sqlite or postgres (default: auto-detect)
  --keep-volumes   Keep the data volume (stop only; by default it is removed)
  -v, --verbose    Print diagnostics (e.g. the sql target volume/network)

Commands:
  start   build (if needed) and run the service with docker compose
  stop    tear the service's compose stack down
  sql     run a SQL query against the running service's database:
            sqlite   runs sqlite3 in a container mounting the data volume
            postgres runs psql in a container on the service's compose network
            (not available for file storage)

If neither --service nor --bin is given, the current directory is used as the
service directory: BINARY_NAME is read from ./Makefile and bin/$BINARY_NAME is
used. fabrun auto-detects the storage type (file or ent) and the db driver from
the service's .fabrica.yaml, then runs the service with docker compose. File
storage services are run with --data-dir /data on a persistent volume.

Environment variables (override fabrun's defaults):
  FABRUN_NAME             Container/service name
  FABRUN_IMAGE            Service image to run (set to skip building one)
  FABRUN_POSTGRES_IMAGE   Postgres image (default: postgres:16-alpine)
  FABRUN_PG_USER          Postgres user (default: postgres)
  FABRUN_PG_PASSWORD      Postgres password (default: postgres)
  FABRUN_PG_PORT          Host port for postgres (default: 5433)
  FABRUN_SQLITE_IMAGE     Image with sqlite3 for 'fabrun sql' (default: keinos/sqlite3)
  FABRUN_HOME             Directory for per-service compose state (default: ~/.fabrun)
`)
}

// parseFlags parses the start/stop/sql flags. It accepts both "--flag value" and
// "--flag=value" forms, and collects any positional arguments into opts.rest.
func parseFlags(args []string) (options, error) {
	var opts options
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, val, hasEq := strings.Cut(arg, "=")

		next := func() (string, error) {
			if hasEq {
				return val, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("flag %s requires a value", name)
			}
			i++
			return args[i], nil
		}

		switch name {
		case "--service":
			v, err := next()
			if err != nil {
				return opts, err
			}
			opts.serviceDir = v
		case "--bin":
			v, err := next()
			if err != nil {
				return opts, err
			}
			opts.binPath = v
		case "--name":
			v, err := next()
			if err != nil {
				return opts, err
			}
			opts.name = v
		case "--port":
			v, err := next()
			if err != nil {
				return opts, err
			}
			opts.port = v
		case "--pg-port":
			v, err := next()
			if err != nil {
				return opts, err
			}
			opts.pgPort = v
		case "--db":
			v, err := next()
			if err != nil {
				return opts, err
			}
			switch dbKind(v) {
			case dbFile, dbSQLite, dbPostgres:
				opts.dbForce = dbKind(v)
			default:
				return opts, fmt.Errorf("invalid --db value %q (want file, sqlite or postgres)", v)
			}
		case "--keep-volumes":
			opts.keepVolumes = true
		case "--auto-ports":
			opts.autoPorts = true
		case "-v", "--verbose":
			opts.verbose = true
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, fmt.Errorf("unknown flag %q", arg)
			}
			opts.rest = append(opts.rest, arg)
		}
	}
	if opts.port == "" {
		opts.port = servicePort
	}
	return opts, nil
}

// resolveBinary determines the path to the service binary and the service
// directory (where .fabrica.yaml lives). It fails if the binary does not exist.
//
// --bin names the executable directly. --service names the service source
// directory, from whose Makefile BINARY_NAME is read and bin/$BINARY_NAME is
// used; a direct executable path is also accepted. With neither flag, the
// current directory is used as the service directory.
func resolveBinary(opts options) (binPath, serviceDir string, err error) {
	if opts.binPath != "" {
		if !fileExists(opts.binPath) {
			return "", "", fmt.Errorf("binary %q does not exist", opts.binPath)
		}
		return opts.binPath, configDirFor(opts.binPath), nil
	}

	dir := opts.serviceDir
	if dir == "" {
		dir = "."
	}

	// Accept a direct executable path passed to --service for convenience.
	if fileExists(dir) {
		return dir, configDirFor(dir), nil
	}
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		return "", "", fmt.Errorf("service path %q does not exist", dir)
	}

	binaryName, err := binaryNameFromMakefile(filepath.Join(dir, "Makefile"))
	if err != nil {
		return "", "", err
	}
	binPath = filepath.Join(dir, "bin", binaryName)
	if !fileExists(binPath) {
		return "", "", fmt.Errorf("%s does not exist (run `make build` in %s first)", binPath, dir)
	}
	return binPath, dir, nil
}

// configDirFor returns the directory that holds the service's .fabrica.yaml for
// a given binary path. Service binaries live at <dir>/bin/<name>, so the
// grandparent directory is checked first, falling back to the current directory.
func configDirFor(binPath string) string {
	dir := filepath.Dir(filepath.Dir(binPath))
	if fileExists(filepath.Join(dir, ".fabrica.yaml")) {
		return dir
	}
	return "."
}

// resolveName returns the service/container name: --name if given, otherwise the
// base name of the binary.
func resolveName(opts options, binPath string) string {
	if opts.name != "" {
		return opts.name
	}
	if v := os.Getenv("FABRUN_NAME"); v != "" {
		return v
	}
	return filepath.Base(binPath)
}

// binaryNameFromMakefile extracts BINARY_NAME=... from a Makefile.
func binaryNameFromMakefile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w (provide --bin instead)", path, err)
	}
	defer f.Close()

	re := regexp.MustCompile(`^\s*BINARY_NAME\s*[:?]?=\s*(.+?)\s*$`)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if m := re.FindStringSubmatch(scanner.Text()); m != nil {
			return strings.TrimSpace(m[1]), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("BINARY_NAME not found in Makefile")
}

// detectDB determines the storage backend. An explicit --db wins. Otherwise it
// reads the service's .fabrica.yaml (features.storage): file storage selects the
// file backend (db_driver is ignored, as fabrica always writes one), and ent
// storage selects sqlite or postgres from db_driver. It defaults to sqlite when
// nothing can be determined.
func detectDB(opts options, serviceDir string) (dbKind, bool, error) {
	if opts.dbForce != "" {
		return opts.dbForce, opts.dbForce != dbFile, nil
	}

	storageType, driver := readFabricaStorage(filepath.Join(serviceDir, ".fabrica.yaml"))
	usesEnt := storageType == "ent"
	switch {
	case storageType == "file":
		return dbFile, false, nil
	case strings.HasPrefix(driver, "postgres"):
		return dbPostgres, usesEnt, nil
	case strings.HasPrefix(driver, "sqlite"):
		return dbSQLite, usesEnt, nil
	default:
		// No config found: assume sqlite, the simplest default.
		return dbSQLite, usesEnt, nil
	}
}

// readFabricaStorage parses .fabrica.yaml and returns the lower-cased
// features.storage.type and features.storage.db_driver values (empty if absent).
func readFabricaStorage(path string) (storageType, driver string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	var cfg struct {
		Features struct {
			Storage struct {
				Type     string `yaml:"type"`
				DBDriver string `yaml:"db_driver"`
			} `yaml:"storage"`
		} `yaml:"features"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", ""
	}
	return strings.ToLower(strings.TrimSpace(cfg.Features.Storage.Type)),
		strings.ToLower(strings.TrimSpace(cfg.Features.Storage.DBDriver))
}

func runStart(opts options) error {
	binPath, serviceDir, err := resolveBinary(opts)
	if err != nil {
		return err
	}
	name := resolveName(opts, binPath)
	db, usesEnt, err := detectDB(opts, serviceDir)
	if err != nil {
		return err
	}

	fmt.Printf("fabrun: service=%s binary=%s ent=%v db=%s\n", name, binPath, usesEnt, db)

	svcPort, pgPort, err := resolvePorts(opts, db)
	if err != nil {
		return err
	}

	image, err := ensureImage(name, binPath, db)
	if err != nil {
		return err
	}

	return composeUp(name, image, db, svcPort, pgPort)
}

// resolvePorts determines the host service port and (for postgres) the host
// postgres port. With --auto-ports it searches for free ports starting at the
// preferred values; otherwise an in-use port is a hard error.
func resolvePorts(opts options, db dbKind) (svcPort, pgPort string, err error) {
	svcPort = opts.port
	if svcPort == "" {
		svcPort = envOr("FABRUN_PORT", servicePort)
	}
	pgPort = opts.pgPort
	if pgPort == "" {
		pgPort = envOr("FABRUN_PG_PORT", defaultPgPort)
	}

	if opts.autoPorts {
		svcPort, err = findFreePort(svcPort)
		if err != nil {
			return "", "", err
		}
		if db == dbPostgres {
			pgPort, err = findFreePort(pgPort)
			if err != nil {
				return "", "", err
			}
		}
		return svcPort, pgPort, nil
	}

	if !portAvailable(svcPort) {
		return "", "", fmt.Errorf("port %s is already in use (use --auto-ports to pick a free one)", svcPort)
	}
	if db == dbPostgres && !portAvailable(pgPort) {
		return "", "", fmt.Errorf("postgres port %s is already in use (use --auto-ports to pick a free one)", pgPort)
	}
	return svcPort, pgPort, nil
}

// ensureImage returns a prebuilt image from FABRUN_IMAGE when set, otherwise it
// builds one from the embedded Dockerfile.
func ensureImage(name, binPath string, db dbKind) (string, error) {
	if img := os.Getenv("FABRUN_IMAGE"); img != "" {
		fmt.Printf("fabrun: using image %s (FABRUN_IMAGE)\n", img)
		return img, nil
	}
	return buildImage(name, binPath, db)
}

// buildImage writes the embedded Dockerfile and the service binary into a temp
// build context and builds the image, returning its tag.
func buildImage(name, binPath string, db dbKind) (string, error) {
	image := "fabrun-" + strings.ToLower(name) + ":latest"

	dockerfile := "Dockerfile." + string(db)
	dfContent, err := assets.ReadFile(dockerfile)
	if err != nil {
		return "", fmt.Errorf("reading embedded %s: %w", dockerfile, err)
	}

	ctxDir, err := os.MkdirTemp("", "fabrun-build-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(ctxDir)

	if err := os.WriteFile(filepath.Join(ctxDir, "Dockerfile"), dfContent, 0o644); err != nil {
		return "", err
	}
	if err := copyFile(binPath, filepath.Join(ctxDir, "service"), 0o755); err != nil {
		return "", fmt.Errorf("copying binary into build context: %w", err)
	}

	fmt.Printf("fabrun: building image %s\n", image)
	if err := runDocker("build", "-t", image, ctxDir); err != nil {
		return "", fmt.Errorf("docker build failed: %w", err)
	}
	return image, nil
}

// composeUp renders the embedded compose file for the backend and brings the
// stack up with `docker compose`. On any failure it tears down whatever it
// started.
func composeUp(name, image string, db dbKind, port, pgPort string) error {
	composeFile, err := writeComposeFile(name, db)
	if err != nil {
		return err
	}
	project := sanitizeProject(name)
	env := composeEnv(name, image, db, port, pgPort)

	fmt.Printf("fabrun: starting %s on port %s (docker compose)\n", name, port)
	if err := runCompose(project, composeFile, env, "up", "-d"); err != nil {
		composeDownQuiet(project, composeFile, env)
		return fmt.Errorf("docker compose up failed: %w", err)
	}
	if err := verifyRunning(name); err != nil {
		composeDownQuiet(project, composeFile, env)
		return err
	}
	fmt.Printf("fabrun: started %s\n", name)
	if db == dbPostgres {
		printPostgresInfo(name, pgPort)
	}
	return nil
}

// composeDownQuiet tears a partially started stack back down after a failure.
func composeDownQuiet(project, composeFile string, env []string) {
	fmt.Printf("fabrun: cleaning up started containers\n")
	_ = runCompose(project, composeFile, env, "down")
}

// printPostgresInfo prints the host-side postgres connection string so the user
// can connect with psql directly.
func printPostgresInfo(name, pgPort string) {
	user := envOr("FABRUN_PG_USER", "postgres")
	pass := envOr("FABRUN_PG_PASSWORD", "postgres")
	dbName := sanitizeDBName(name)
	pg := fmt.Sprintf("postgres://%s:%s@localhost:%s/%s?sslmode=disable", user, pass, pgPort, dbName)
	fmt.Printf("fabrun: postgres published on localhost:%s\n", pgPort)
	fmt.Printf("  export PG=%q\n", pg)
	fmt.Printf("  psql \"$PG\" -c '\\dt'\n")
}

// writeComposeFile writes the embedded compose file for the backend into the
// service's state directory and returns its path.
func writeComposeFile(name string, db dbKind) (string, error) {
	src := "compose." + string(db) + ".yml"
	content, err := assets.ReadFile(src)
	if err != nil {
		return "", fmt.Errorf("reading embedded %s: %w", src, err)
	}
	dir, err := stateDir(name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "compose.yml")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// composeEnv builds the environment for a compose invocation. User-provided
// values (e.g. FABRUN_POSTGRES_IMAGE) are preserved; fabrun fills in the rest.
func composeEnv(name, image string, db dbKind, port, pgPort string) []string {
	m := map[string]string{}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	m["FABRUN_NAME"] = name
	m["FABRUN_IMAGE"] = image
	m["FABRUN_PORT"] = port

	switch db {
	case dbFile:
		m["FABRUN_DATA_DIR"] = "/data"
	case dbPostgres:
		if m["FABRUN_POSTGRES_IMAGE"] == "" {
			m["FABRUN_POSTGRES_IMAGE"] = "postgres:16-alpine"
		}
		if m["FABRUN_PG_USER"] == "" {
			m["FABRUN_PG_USER"] = "postgres"
		}
		if m["FABRUN_PG_PASSWORD"] == "" {
			m["FABRUN_PG_PASSWORD"] = "postgres"
		}
		m["FABRUN_PG_PORT"] = pgPort
		m["FABRUN_PG_DB"] = sanitizeDBName(name)
		m["FABRUN_DATABASE_URL"] = fmt.Sprintf("postgres://%s:%s@postgres:5432/%s?sslmode=disable",
			m["FABRUN_PG_USER"], m["FABRUN_PG_PASSWORD"], m["FABRUN_PG_DB"])
	default:
		m["FABRUN_DATABASE_URL"] = fmt.Sprintf("file:/data/%s.db?_fk=1", name)
	}

	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}

// runCompose runs `docker compose` for the given project and compose file.
func runCompose(project, composeFile string, env []string, args ...string) error {
	full := append([]string{"compose", "-p", project, "-f", composeFile}, args...)
	cmd := exec.Command("docker", full...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// stateDir returns the per-service directory holding its compose file. It
// honours FABRUN_HOME, defaulting to ~/.fabrun.
func stateDir(name string) (string, error) {
	base := os.Getenv("FABRUN_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".fabrun")
	}
	return filepath.Join(base, sanitizeProject(name)), nil
}

// sanitizeProject converts a service name into a valid compose project name.
func sanitizeProject(name string) string {
	s := strings.ToLower(name)
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		default:
			return '-'
		}
	}, s)
	return strings.TrimLeft(s, "_-")
}

// verifyRunning gives a freshly started container a moment to settle and fails
// (dumping its logs) if it has already exited.
func verifyRunning(name string) error {
	deadline := time.Now().Add(3 * time.Second)
	for {
		running, exitCode, err := containerState(name)
		if err != nil {
			return fmt.Errorf("could not inspect container %s: %w", name, err)
		}
		if !running {
			logs := containerLogs(name)
			return fmt.Errorf("container %s exited (code %d) right after start; logs:\n%s", name, exitCode, logs)
		}
		if time.Now().After(deadline) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// containerState reports whether a container is running and its last exit code.
func containerState(name string) (running bool, exitCode int, err error) {
	out, err := exec.Command("docker", "inspect",
		"-f", "{{.State.Running}} {{.State.ExitCode}}", name).Output()
	if err != nil {
		return false, 0, err
	}
	_, scanErr := fmt.Sscanf(strings.TrimSpace(string(out)), "%t %d", &running, &exitCode)
	if scanErr != nil {
		return false, 0, scanErr
	}
	return running, exitCode, nil
}

// containerLogs returns the tail of a container's logs for diagnostics.
func containerLogs(name string) string {
	out, _ := exec.Command("docker", "logs", "--tail", "50", name).CombinedOutput()
	return strings.TrimRight(string(out), "\n")
}

func runStop(opts options) error {
	// For stop, a binary is not strictly required; fall back to Makefile/bin
	// only when neither --name nor --bin is provided.
	name, err := serviceName(opts)
	if err != nil {
		return err
	}

	project := sanitizeProject(name)
	dir, err := stateDir(name)
	if err != nil {
		return err
	}
	composeFile := filepath.Join(dir, "compose.yml")

	fmt.Printf("fabrun: stopping %s (docker compose down)\n", name)

	if fileExists(composeFile) {
		// Detect the backend from the stored compose file so compose can
		// interpolate every variable without warnings.
		data, _ := os.ReadFile(composeFile)
		db := dbFromCompose(data)
		image := os.Getenv("FABRUN_IMAGE")
		if image == "" {
			image = "fabrun-" + strings.ToLower(name) + ":latest"
		}
		env := composeEnv(name, image, db, servicePort, defaultPgPort)

		args := []string{"down"}
		if !opts.keepVolumes {
			args = append(args, "--volumes")
		}
		if err := runCompose(project, composeFile, env, args...); err != nil {
			return fmt.Errorf("docker compose down failed: %w", err)
		}
		_ = os.RemoveAll(dir)
	} else {
		// No stored compose file: tear down best-effort by container name.
		removeContainer(name)
		removeContainer(name + "-postgres")
		if !opts.keepVolumes {
			removeVolume(project + "_data")
		}
	}

	if opts.keepVolumes {
		fmt.Printf("fabrun: kept volumes\n")
	}
	fmt.Printf("fabrun: stopped %s\n", name)
	return nil
}

// serviceName resolves the service/container name the same way start does:
// --name, then FABRUN_NAME, then the service binary's base name.
func serviceName(opts options) (string, error) {
	if opts.name != "" {
		return opts.name, nil
	}
	if v := os.Getenv("FABRUN_NAME"); v != "" {
		return v, nil
	}
	binPath, _, err := resolveBinary(opts)
	if err != nil {
		return "", err
	}
	return filepath.Base(binPath), nil
}

// runSQL runs a single SQL query against the running service's database. It
// resolves the service name and backend exactly like start, then dispatches to
// the sqlite or postgres path.
func runSQL(opts options, query string) error {
	name, err := serviceName(opts)
	if err != nil {
		return err
	}
	db := detectRunningDB(opts, name)
	project := sanitizeProject(name)

	switch db {
	case dbPostgres:
		return sqlPostgres(name, project, query, opts.verbose)
	case dbFile:
		return fmt.Errorf("%s uses file storage, which has no SQL database; its data is in docker volume %s_data (at /data)", name, project)
	}
	return sqlSQLite(name, project, query, opts.verbose)
}

// dbFromCompose identifies the backend of a rendered (stored) compose file.
func dbFromCompose(data []byte) dbKind {
	s := string(data)
	switch {
	case strings.Contains(s, "FABRUN_POSTGRES_IMAGE"):
		return dbPostgres
	case strings.Contains(s, "FABRUN_DATA_DIR"):
		return dbFile
	default:
		return dbSQLite
	}
}

// detectRunningDB determines the backend to query. An explicit --db wins; then
// the stored compose file (authoritative for what is actually running); then the
// service's .fabrica.yaml; defaulting to sqlite.
func detectRunningDB(opts options, name string) dbKind {
	if opts.dbForce != "" {
		return opts.dbForce
	}
	if dir, err := stateDir(name); err == nil {
		if data, err := os.ReadFile(filepath.Join(dir, "compose.yml")); err == nil {
			return dbFromCompose(data)
		}
	}
	if _, serviceDir, err := resolveBinary(opts); err == nil {
		if db, _, err := detectDB(opts, serviceDir); err == nil {
			return db
		}
	}
	return dbSQLite
}

// sqlSQLite runs sqlite3 in a throwaway container that mounts the service's data
// volume, pointing at the same db file the service uses.
func sqlSQLite(name, project, query string, verbose bool) error {
	volume := project + "_data"
	dbFile := fmt.Sprintf("/data/%s.db", name)
	image := envOr("FABRUN_SQLITE_IMAGE", "keinos/sqlite3")
	args := []string{"run", "--rm", "-i",
		"-v", volume + ":/data",
		"--entrypoint", "sqlite3",
		image, dbFile, query}
	if verbose {
		fmt.Fprintf(os.Stderr, "fabrun: sqlite query on %s (volume %s)\n", dbFile, volume)
		fmt.Fprintf(os.Stderr, "+ %s\n", shellJoin("docker", args))
	}
	return runDocker(args...)
}

// sqlPostgres runs psql in a throwaway container attached to the service's
// compose network, connecting to the postgres service by its compose hostname.
func sqlPostgres(name, project, query string, verbose bool) error {
	network := project + "_default"
	user := envOr("FABRUN_PG_USER", "postgres")
	pass := envOr("FABRUN_PG_PASSWORD", "postgres")
	dbName := sanitizeDBName(name)
	image := envOr("FABRUN_POSTGRES_IMAGE", "postgres:16-alpine")
	url := fmt.Sprintf("postgres://%s:%s@postgres:5432/%s?sslmode=disable", user, pass, dbName)
	args := []string{"run", "--rm", "-i",
		"--network", network,
		image, "psql", url, "-c", query}
	if verbose {
		fmt.Fprintf(os.Stderr, "fabrun: postgres query on db %s (network %s)\n", dbName, network)
		fmt.Fprintf(os.Stderr, "+ %s\n", shellJoin("docker", args))
	}
	return runDocker(args...)
}

// shellJoin renders a command and its arguments as a single copy-pasteable
// shell line, quoting any argument that needs it.
func shellJoin(name string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, shellQuote(name))
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " ")
}

// shellQuote single-quotes a string for POSIX shells when it contains anything
// outside a safe unquoted set.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			strings.ContainsRune("@%_-+=:,./", r) {
			continue
		}
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}

// ---- docker helpers ----

func runDocker(args ...string) error {
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// removeContainer stops and removes a container, ignoring errors (it may not
// exist).
func removeContainer(name string) {
	_ = exec.Command("docker", "rm", "-f", name).Run()
}

// removeVolume removes a named docker volume, ignoring errors (it may not exist).
func removeVolume(name string) {
	_ = exec.Command("docker", "volume", "rm", name).Run()
}

// ---- small utilities ----

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// envOr returns the environment value for key, or def when it is unset/empty.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// portAvailable reports whether a TCP port can be bound on IPv4. Published
// docker ports bind the IPv4 wildcard, so this reliably detects conflicts.
func portAvailable(port string) bool {
	ln, err := net.Listen("tcp4", "0.0.0.0:"+port)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// findFreePort returns the first available port at or above start.
func findFreePort(start string) (string, error) {
	n, err := strconv.Atoi(start)
	if err != nil {
		return "", fmt.Errorf("invalid port %q", start)
	}
	for p := n; p <= n+1000 && p <= 65535; p++ {
		if portAvailable(strconv.Itoa(p)) {
			return strconv.Itoa(p), nil
		}
	}
	return "", fmt.Errorf("no free port found at or above %d", n)
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, in, mode)
}

// sanitizeDBName converts a service name into a valid postgres identifier.
func sanitizeDBName(name string) string {
	s := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, name)
	if s == "" || (s[0] >= '0' && s[0] <= '9') {
		s = "db_" + s
	}
	return strings.ToLower(s)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "fabrun: %v\n", err)
	os.Exit(1)
}
