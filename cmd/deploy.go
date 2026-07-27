package cmd

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	foundrydb "github.com/foundrydb/foundrydb-sdk-go/foundrydb"
	"github.com/spf13/cobra"
)

// foundry deploy: ship the current directory.
//
// The platform builds from source, so the CLI's job is to get the source there
// and then stay out of the way. It packs the directory, uploads it, and either
// creates the app or rebuilds an existing one from the new source.
//
// Two properties are worth stating because they are what make this safe to run
// from a working tree. Nothing is uploaded that the customer would not expect:
// .git, node_modules and friends are excluded by default, so a deploy does not
// quietly ship history or a gigabyte of dependencies. And the tarball is packed
// under a single top-level directory, which is the layout the builder strips,
// so a Dockerfile at the root of the directory lands at the root of the build
// context.

const (
	// deployTarRoot is the single wrapping directory the builder strips. The
	// platform documents this packing convention; matching it here is what
	// makes `foundry deploy` and a hand-rolled `tar czf` behave identically.
	deployTarRoot = "src"

	// maxDeployBytes mirrors the API's upload limit. Checked locally so a
	// too-large directory fails immediately with something actionable rather
	// than after uploading tens of megabytes.
	maxDeployBytes int64 = 64 * 1024 * 1024
)

// defaultDeployExcludes are directories never worth uploading. They are build
// or VCS artifacts the platform rebuilds or does not want: shipping them wastes
// the customer's upload, and .git in particular would put source history into a
// build context that ends up in an image layer.
var defaultDeployExcludes = []string{
	".git", "node_modules", ".next", "dist", "build", "target",
	"vendor", "__pycache__", ".venv", "venv", ".terraform", ".DS_Store",
}

var deployCmd = &cobra.Command{
	Use:   "deploy [directory]",
	Short: "Deploy a local directory: pack it, upload it, and build it on the platform",
	Long: `Deploy the current directory (or the one given) as an app.

The directory is packed into a gzipped tarball, uploaded, and built on the
platform: with your Dockerfile if there is one, with Cloud Native Buildpacks if
there is not. Use this when the code is not somewhere the platform can clone
from; for a git repository, point the app at the repository instead and let a
push deploy it.

  foundry deploy                        # create or update an app from this directory
  foundry deploy ./service --name api   # deploy a subdirectory as "api"
  foundry deploy --app <id>             # rebuild an existing app from this directory

Common build artifacts (.git, node_modules, dist, target, ...) are excluded.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().String("name", "", "App name (defaults to the directory name; only used when creating)")
	deployCmd.Flags().String("app", "", "Deploy into this existing app id instead of creating one")
	deployCmd.Flags().Int("port", 8080, "Port your container listens on")
	deployCmd.Flags().String("plan", "tier-2", "Compute plan")
	deployCmd.Flags().String("zone", "se-sto1", "Zone")
	deployCmd.Flags().String("builder", "", "Build strategy: empty auto-detects, or \"dockerfile\" / \"buildpacks\"")
	deployCmd.Flags().String("scan-policy", "", "Vulnerability policy: empty records findings, or \"block-critical\" / \"block-high\"")
	deployCmd.Flags().StringSlice("exclude", nil, "Additional paths to exclude, on top of the defaults")
	deployCmd.Flags().Bool("wait", false, "Wait until the app is Running")
	rootCmd.AddCommand(deployCmd)
}

func runDeploy(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) == 1 {
		dir = args[0]
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", dir, err)
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}

	extra, _ := cmd.Flags().GetStringSlice("exclude")
	excludes := append(append([]string{}, defaultDeployExcludes...), extra...)

	fmt.Printf("Packing %s\n", abs)
	tarball, packed, err := packDirectory(abs, excludes)
	if err != nil {
		return err
	}
	defer os.Remove(tarball)
	if packed == 0 {
		return fmt.Errorf("%s contains no files to deploy after exclusions", dir)
	}

	size, err := fileSize(tarball)
	if err != nil {
		return err
	}
	if size > maxDeployBytes {
		return fmt.Errorf("packed source is %s, above the %s limit; exclude more with --exclude",
			humanBytes(size), humanBytes(maxDeployBytes))
	}
	fmt.Printf("Packed %d files (%s)\n", packed, humanBytes(size))

	f, err := os.Open(tarball)
	if err != nil {
		return fmt.Errorf("open packed source: %w", err)
	}
	defer f.Close()

	c := newClient()
	ctx := context.Background()

	fmt.Print("Uploading... ")
	up, err := c.UploadAppSource(ctx, f)
	if err != nil {
		fmt.Println()
		return fmt.Errorf("upload failed: %w", err)
	}
	fmt.Printf("done (%s)\n", up.ChecksumSHA256[:12])

	appID, _ := cmd.Flags().GetString("app")
	if appID != "" {
		return deployIntoExisting(ctx, cmd, c, appID, up)
	}
	return deployAsNewApp(ctx, cmd, c, abs, up)
}

// deployAsNewApp creates an app whose source is the upload just made.
func deployAsNewApp(ctx context.Context, cmd *cobra.Command, c *foundrydb.Client, dir string, up *foundrydb.AppSourceUpload) error {
	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		name = sanitiseAppName(filepath.Base(dir))
	}
	port, _ := cmd.Flags().GetInt("port")
	plan, _ := cmd.Flags().GetString("plan")
	zone, _ := cmd.Flags().GetString("zone")
	builder, _ := cmd.Flags().GetString("builder")
	policy, _ := cmd.Flags().GetString("scan-policy")

	req := foundrydb.CreateAppServiceRequest{
		Name:     name,
		PlanName: plan,
		Zone:     zone,
		AppConfig: foundrydb.AppContainerConfig{
			ContainerPort: port,
			Source: &foundrydb.AppSource{
				Type:       "upload",
				UploadRef:  up.UploadRef,
				Builder:    builder,
				ScanPolicy: foundrydb.AppScanPolicy(policy),
			},
		},
	}

	fmt.Printf("Creating app %q in %s...\n", name, zone)
	app, err := c.CreateAppService(ctx, req)
	if err != nil {
		return fmt.Errorf("create app: %w", err)
	}
	fmt.Printf("Created %s\n", app.ID)
	return finishDeploy(ctx, cmd, c, app.ID)
}

// deployIntoExisting points an existing app at the new upload and rebuilds it.
//
// This deliberately PATCHes app_config.source rather than calling
// UpdateAppBuildSettings. The build-settings endpoint preserves the existing
// upload reference by design (its whole point is editing build inputs without
// changing how source is ingested), so using it here uploaded new source and
// then rebuilt the PREVIOUS upload: the command reported success, the app
// reached Running, and the customer's new code silently never shipped. That is
// worse than an error, so the new reference is sent explicitly.
func deployIntoExisting(ctx context.Context, cmd *cobra.Command, c *foundrydb.Client, appID string, up *foundrydb.AppSourceUpload) error {
	builder, _ := cmd.Flags().GetString("builder")
	policy, _ := cmd.Flags().GetString("scan-policy")

	current, err := c.GetAppService(ctx, appID)
	if err != nil {
		return fmt.Errorf("load app %s: %w", appID, err)
	}
	if current == nil {
		return fmt.Errorf("app %s not found", appID)
	}
	if current.AppConfig == nil {
		return fmt.Errorf("app %s has no container configuration to update", appID)
	}
	// The port is required on update and must keep its current value, so it is
	// carried over rather than re-specified from a flag default.
	cfg := *current.AppConfig
	// Registry credentials are write-only: a read returns the username with the
	// password redacted, so echoing the pair straight back fails validation
	// ("must be provided together"). Clearing both leaves the stored values
	// untouched, which is the documented behaviour for omitted secrets.
	cfg.RegistryUsername = ""
	cfg.RegistryPassword = ""
	cfg.Source = &foundrydb.AppSource{
		Type:       "upload",
		UploadRef:  up.UploadRef,
		Builder:    builder,
		ScanPolicy: foundrydb.AppScanPolicy(policy),
	}

	fmt.Printf("Pointing %s at the new source...\n", appID)
	if _, err := c.UpdateAppService(ctx, appID, foundrydb.UpdateAppServiceRequest{AppConfig: cfg}); err != nil {
		return fmt.Errorf("point app at new source: %w", err)
	}
	return finishDeploy(ctx, cmd, c, appID)
}

// finishDeploy optionally waits for the app to come up, and always tells the
// caller how to follow it if they would rather not wait.
func finishDeploy(ctx context.Context, cmd *cobra.Command, c *foundrydb.Client, appID string) error {
	wait, _ := cmd.Flags().GetBool("wait")
	if !wait {
		fmt.Printf("\nBuilding. Follow it with:\n  foundry apps get %s\n", appID)
		return nil
	}

	fmt.Print("Waiting for the app to become Running")
	deadline := time.Now().Add(20 * time.Minute)
	for time.Now().Before(deadline) {
		app, err := c.GetAppService(ctx, appID)
		if err == nil && app != nil {
			switch {
			case app.Status == "Running":
				fmt.Printf("\nRunning: %s\n", app.URL)
				return nil
			case strings.HasSuffix(app.Status, "Failed"):
				// Surface the build's own cause rather than just the status:
				// a failed build is almost always explained by its last step.
				fmt.Printf("\nDeploy failed (%s)\n", app.Status)
				if builds, bErr := c.ListAppBuilds(ctx, appID, 1); bErr == nil && len(builds) > 0 && builds[0].ErrorMessage != "" {
					fmt.Printf("Build error: %s\n", builds[0].ErrorMessage)
				}
				return fmt.Errorf("deploy failed")
			}
		}
		fmt.Print(".")
		time.Sleep(10 * time.Second)
	}
	fmt.Println()
	return fmt.Errorf("timed out waiting for the app to become Running; it may still be building")
}

// packDirectory writes a gzipped tarball of dir and returns its path and the
// number of files packed. Everything is rooted under deployTarRoot so the
// builder's single-directory strip lands the source at the build root.
func packDirectory(dir string, excludes []string) (string, int, error) {
	out, err := os.CreateTemp("", "foundry-deploy-*.tar.gz")
	if err != nil {
		return "", 0, fmt.Errorf("create temporary archive: %w", err)
	}
	path := out.Name()

	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)

	count := 0
	walkErr := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(dir, p)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if isExcluded(rel, excludes) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Symlinks are skipped rather than followed: following them can escape
		// the directory being deployed, and archiving them as links would ship
		// paths that do not exist on the build host.
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		hdr, hErr := tar.FileInfoHeader(info, "")
		if hErr != nil {
			return hErr
		}
		hdr.Name = filepath.ToSlash(filepath.Join(deployTarRoot, rel))
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		src, oErr := os.Open(p)
		if oErr != nil {
			return oErr
		}
		defer src.Close()
		if _, cErr := io.Copy(tw, src); cErr != nil {
			return cErr
		}
		count++
		return nil
	})

	if err := tw.Close(); err != nil && walkErr == nil {
		walkErr = err
	}
	if err := gz.Close(); err != nil && walkErr == nil {
		walkErr = err
	}
	if err := out.Close(); err != nil && walkErr == nil {
		walkErr = err
	}
	if walkErr != nil {
		os.Remove(path)
		return "", 0, fmt.Errorf("pack %s: %w", dir, walkErr)
	}
	return path, count, nil
}

// isExcluded reports whether a repo-relative path is excluded. Matching is on
// path segments so "dist" excludes ./dist and ./web/dist, but never a file
// merely containing that text in its name.
func isExcluded(rel string, excludes []string) bool {
	segs := strings.Split(filepath.ToSlash(rel), "/")
	for _, ex := range excludes {
		ex = strings.Trim(strings.TrimSpace(ex), "/")
		if ex == "" {
			continue
		}
		for _, s := range segs {
			if s == ex {
				return true
			}
		}
	}
	return false
}

// sanitiseAppName turns a directory name into something the platform accepts:
// lowercase, starting with a letter, alphanumeric or dashes.
func sanitiseAppName(base string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(base) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.' || r == ' ':
			b.WriteRune('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" || !(name[0] >= 'a' && name[0] <= 'z') {
		name = "app-" + name
	}
	if len(name) > 40 {
		name = strings.Trim(name[:40], "-")
	}
	return name
}

func fileSize(path string) (int64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("stat archive: %w", err)
	}
	return fi.Size(), nil
}

func humanBytes(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}
