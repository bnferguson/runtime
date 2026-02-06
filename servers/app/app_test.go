package app

import (
	"context"
	"log/slog"
	"testing"

	"miren.dev/runtime/api/app/app_v1alpha"
	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/api/entityserver"
	"miren.dev/runtime/metrics"
	"miren.dev/runtime/pkg/entity/testutils"
	"miren.dev/runtime/pkg/rpc"
)

func TestSetConfiguration_DuplicateEnvVars(t *testing.T) {
	ctx := context.Background()

	// Setup in-memory entity server
	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ec := entityserver.NewClient(slog.Default(), inmem.EAC)

	// Create AppInfo instance
	appInfo := &AppInfo{
		Log:  slog.Default(),
		EC:   ec,
		CPU:  &metrics.CPUUsage{},
		Mem:  &metrics.MemoryUsage{},
		HTTP: &metrics.HTTPMetrics{},
	}

	// Create RPC client using LocalClient
	client := &app_v1alpha.CrudClient{
		Client: rpc.LocalClient(app_v1alpha.AdaptCrud(appInfo)),
	}

	// Create a test app
	appName := "test-app"
	app := &core_v1alpha.App{}
	appID, err := inmem.Client.Create(ctx, appName, app)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	app.ID = appID

	// Test case 1: Set initial env vars
	t.Run("InitialEnvVars", func(t *testing.T) {
		// Build configuration with new setter methods
		env1 := &app_v1alpha.NamedValue{}
		env1.SetKey("FOO")
		env1.SetValue("bar")
		env1.SetSensitive(false)

		env2 := &app_v1alpha.NamedValue{}
		env2.SetKey("SECRET")
		env2.SetValue("hidden")
		env2.SetSensitive(true)

		cfg := &app_v1alpha.Configuration{}
		cfg.SetEnvVars([]*app_v1alpha.NamedValue{env1, env2})

		// Use the client to set configuration
		_, err := client.SetConfiguration(ctx, appName, cfg)
		if err != nil {
			t.Fatalf("failed to set configuration: %v", err)
		}

		// Verify the configuration by checking the entity directly
		var appCheck core_v1alpha.App
		err = ec.Get(ctx, appName, &appCheck)
		if err != nil {
			t.Fatalf("failed to get app: %v", err)
		}

		var appVerCheck core_v1alpha.AppVersion
		err = ec.GetById(ctx, appCheck.ActiveVersion, &appVerCheck)
		if err != nil {
			t.Fatalf("failed to get app version: %v", err)
		}

		if len(appVerCheck.Config.Variable) != 2 {
			t.Errorf("expected 2 env vars, got %d", len(appVerCheck.Config.Variable))
		}
	})

	// Test case 2: Add duplicate with different sensitive flag
	t.Run("DuplicateWithDifferentSensitiveFlag", func(t *testing.T) {
		// Add FOO again but as sensitive - should replace the existing FOO
		env1 := &app_v1alpha.NamedValue{}
		env1.SetKey("FOO")
		env1.SetValue("secret-bar")
		env1.SetSensitive(true)

		cfg := &app_v1alpha.Configuration{}
		cfg.SetEnvVars([]*app_v1alpha.NamedValue{env1})

		// Use the client to set configuration
		_, err := client.SetConfiguration(ctx, appName, cfg)
		if err != nil {
			t.Fatalf("failed to set configuration: %v", err)
		}

		// Get configuration and check for duplicates
		var appCheck core_v1alpha.App
		err = ec.Get(ctx, appName, &appCheck)
		if err != nil {
			t.Fatalf("failed to get app: %v", err)
		}

		var appVerCheck core_v1alpha.AppVersion
		err = ec.GetById(ctx, appCheck.ActiveVersion, &appVerCheck)
		if err != nil {
			t.Fatalf("failed to get app version: %v", err)
		}

		envVars := appVerCheck.Config.Variable

		// Count FOO occurrences
		fooCount := 0
		for _, ev := range envVars {
			if ev.Key == "FOO" {
				fooCount++
				t.Logf("Found FOO: value=%s, sensitive=%v", ev.Value, ev.Sensitive)
			}
		}

		// With the fix, we should only have 1 FOO (the updated one)
		if fooCount != 1 {
			t.Errorf("Found %d instances of FOO env var (expected 1)", fooCount)
			t.Logf("Total env vars: %d", len(envVars))
			for i, ev := range envVars {
				t.Logf("  [%d] %s = %s (sensitive: %v)", i, ev.Key, ev.Value, ev.Sensitive)
			}
		}
	})

	// Test case 3: Add duplicate with same sensitive flag but different value
	t.Run("DuplicateWithSameSensitiveFlag", func(t *testing.T) {
		// Add SECRET again with different value but same sensitive flag
		env1 := &app_v1alpha.NamedValue{}
		env1.SetKey("SECRET")
		env1.SetValue("updated-secret")
		env1.SetSensitive(true)

		cfg := &app_v1alpha.Configuration{}
		cfg.SetEnvVars([]*app_v1alpha.NamedValue{env1})

		// Use the client to set configuration
		_, err := client.SetConfiguration(ctx, appName, cfg)
		if err != nil {
			t.Fatalf("failed to set configuration: %v", err)
		}

		// Get configuration and check for duplicates
		var appCheck core_v1alpha.App
		err = ec.Get(ctx, appName, &appCheck)
		if err != nil {
			t.Fatalf("failed to get app: %v", err)
		}

		var appVerCheck core_v1alpha.AppVersion
		err = ec.GetById(ctx, appCheck.ActiveVersion, &appVerCheck)
		if err != nil {
			t.Fatalf("failed to get app version: %v", err)
		}

		envVars := appVerCheck.Config.Variable

		// Count SECRET occurrences
		secretCount := 0
		for _, ev := range envVars {
			if ev.Key == "SECRET" {
				secretCount++
				t.Logf("Found SECRET: value=%s, sensitive=%v", ev.Value, ev.Sensitive)
			}
		}

		// With the fix for same key+sensitive, this should work correctly
		if secretCount > 1 {
			t.Errorf("Found %d instances of SECRET env var (expected 1)", secretCount)
		}
	})
}

func TestSetConfiguration_EnvVarDeletion(t *testing.T) {
	ctx := context.Background()

	// Setup in-memory entity server
	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ec := entityserver.NewClient(slog.Default(), inmem.EAC)

	// Create AppInfo instance
	appInfo := &AppInfo{
		Log:  slog.Default(),
		EC:   ec,
		CPU:  &metrics.CPUUsage{},
		Mem:  &metrics.MemoryUsage{},
		HTTP: &metrics.HTTPMetrics{},
	}

	// Create RPC client using LocalClient
	client := &app_v1alpha.CrudClient{
		Client: rpc.LocalClient(app_v1alpha.AdaptCrud(appInfo)),
	}

	// Create a test app
	appName := "test-app-delete"
	app := &core_v1alpha.App{}
	appID, err := inmem.Client.Create(ctx, appName, app)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	app.ID = appID

	// Step 1: Set initial env vars
	t.Run("SetInitialVars", func(t *testing.T) {
		env1 := &app_v1alpha.NamedValue{}
		env1.SetKey("VAR1")
		env1.SetValue("value1")
		env1.SetSensitive(false)

		env2 := &app_v1alpha.NamedValue{}
		env2.SetKey("VAR2")
		env2.SetValue("value2")
		env2.SetSensitive(false)

		env3 := &app_v1alpha.NamedValue{}
		env3.SetKey("VAR3")
		env3.SetValue("value3")
		env3.SetSensitive(true)

		cfg := &app_v1alpha.Configuration{}
		cfg.SetEnvVars([]*app_v1alpha.NamedValue{env1, env2, env3})

		_, err := client.SetConfiguration(ctx, appName, cfg)
		if err != nil {
			t.Fatalf("failed to set initial configuration: %v", err)
		}

		// Verify all 3 vars are set
		var appCheck core_v1alpha.App
		err = ec.Get(ctx, appName, &appCheck)
		if err != nil {
			t.Fatalf("failed to get app: %v", err)
		}

		var appVerCheck core_v1alpha.AppVersion
		err = ec.GetById(ctx, appCheck.ActiveVersion, &appVerCheck)
		if err != nil {
			t.Fatalf("failed to get app version: %v", err)
		}

		if len(appVerCheck.Config.Variable) != 3 {
			t.Errorf("expected 3 env vars, got %d", len(appVerCheck.Config.Variable))
		}
	})

	// Step 2: Delete one var (VAR2)
	t.Run("DeleteOneVar", func(t *testing.T) {
		// Send only VAR1 and VAR3, effectively deleting VAR2
		env1 := &app_v1alpha.NamedValue{}
		env1.SetKey("VAR1")
		env1.SetValue("value1")
		env1.SetSensitive(false)

		env3 := &app_v1alpha.NamedValue{}
		env3.SetKey("VAR3")
		env3.SetValue("value3")
		env3.SetSensitive(true)

		cfg := &app_v1alpha.Configuration{}
		cfg.SetEnvVars([]*app_v1alpha.NamedValue{env1, env3})

		_, err := client.SetConfiguration(ctx, appName, cfg)
		if err != nil {
			t.Fatalf("failed to set configuration after deletion: %v", err)
		}

		// Verify VAR2 is gone
		var appCheck core_v1alpha.App
		err = ec.Get(ctx, appName, &appCheck)
		if err != nil {
			t.Fatalf("failed to get app: %v", err)
		}

		var appVerCheck core_v1alpha.AppVersion
		err = ec.GetById(ctx, appCheck.ActiveVersion, &appVerCheck)
		if err != nil {
			t.Fatalf("failed to get app version: %v", err)
		}

		if len(appVerCheck.Config.Variable) != 2 {
			t.Errorf("expected 2 env vars after deletion, got %d", len(appVerCheck.Config.Variable))
		}

		// Check that VAR2 is specifically gone
		for _, ev := range appVerCheck.Config.Variable {
			if ev.Key == "VAR2" {
				t.Errorf("VAR2 should have been deleted but still exists")
			}
		}

		// Check that VAR1 and VAR3 still exist
		hasVar1, hasVar3 := false, false
		for _, ev := range appVerCheck.Config.Variable {
			if ev.Key == "VAR1" && ev.Value == "value1" {
				hasVar1 = true
			}
			if ev.Key == "VAR3" && ev.Value == "value3" {
				hasVar3 = true
			}
		}
		if !hasVar1 {
			t.Error("VAR1 should still exist after deletion of VAR2")
		}
		if !hasVar3 {
			t.Error("VAR3 should still exist after deletion of VAR2")
		}
	})

	// Step 3: Delete all vars
	t.Run("DeleteAllVars", func(t *testing.T) {
		// Send empty env var list
		cfg := &app_v1alpha.Configuration{}
		cfg.SetEnvVars([]*app_v1alpha.NamedValue{})

		_, err := client.SetConfiguration(ctx, appName, cfg)
		if err != nil {
			t.Fatalf("failed to set configuration with empty vars: %v", err)
		}

		// Verify all vars are gone
		var appCheck core_v1alpha.App
		err = ec.Get(ctx, appName, &appCheck)
		if err != nil {
			t.Fatalf("failed to get app: %v", err)
		}

		var appVerCheck core_v1alpha.AppVersion
		err = ec.GetById(ctx, appCheck.ActiveVersion, &appVerCheck)
		if err != nil {
			t.Fatalf("failed to get app version: %v", err)
		}

		if len(appVerCheck.Config.Variable) != 0 {
			t.Errorf("expected 0 env vars after deleting all, got %d", len(appVerCheck.Config.Variable))
			for _, ev := range appVerCheck.Config.Variable {
				t.Errorf("  unexpected var: %s = %s", ev.Key, ev.Value)
			}
		}
	})
}

func TestSetEnvVars_Batch(t *testing.T) {
	ctx := context.Background()

	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ec := entityserver.NewClient(slog.Default(), inmem.EAC)

	appInfo := &AppInfo{
		Log:  slog.Default(),
		EC:   ec,
		CPU:  &metrics.CPUUsage{},
		Mem:  &metrics.MemoryUsage{},
		HTTP: &metrics.HTTPMetrics{},
	}

	client := &app_v1alpha.CrudClient{
		Client: rpc.LocalClient(app_v1alpha.AdaptCrud(appInfo)),
	}

	t.Run("SetMultipleVarsAtOnce", func(t *testing.T) {
		appName := "test-batch-set"
		app := &core_v1alpha.App{}
		appID, err := inmem.Client.Create(ctx, appName, app)
		if err != nil {
			t.Fatalf("failed to create app: %v", err)
		}
		app.ID = appID

		nv1 := &app_v1alpha.NamedValue{}
		nv1.SetKey("A")
		nv1.SetValue("1")
		nv1.SetSensitive(false)

		nv2 := &app_v1alpha.NamedValue{}
		nv2.SetKey("B")
		nv2.SetValue("2")
		nv2.SetSensitive(false)

		nv3 := &app_v1alpha.NamedValue{}
		nv3.SetKey("C")
		nv3.SetValue("3")
		nv3.SetSensitive(true)

		res, err := client.SetEnvVars(ctx, appName, []*app_v1alpha.NamedValue{nv1, nv2, nv3}, "")
		if err != nil {
			t.Fatalf("SetEnvVars failed: %v", err)
		}

		if res.VersionId() == "" {
			t.Fatal("expected non-empty version ID")
		}

		var appCheck core_v1alpha.App
		err = ec.Get(ctx, appName, &appCheck)
		if err != nil {
			t.Fatalf("failed to get app: %v", err)
		}

		var appVer core_v1alpha.AppVersion
		err = ec.GetById(ctx, appCheck.ActiveVersion, &appVer)
		if err != nil {
			t.Fatalf("failed to get app version: %v", err)
		}

		if len(appVer.Config.Variable) != 3 {
			t.Fatalf("expected 3 env vars, got %d", len(appVer.Config.Variable))
		}

		vars := map[string]core_v1alpha.Variable{}
		for _, v := range appVer.Config.Variable {
			vars[v.Key] = v
		}

		if vars["A"].Value != "1" || vars["A"].Sensitive {
			t.Errorf("A: got value=%q sensitive=%v, want value=%q sensitive=%v", vars["A"].Value, vars["A"].Sensitive, "1", false)
		}
		if vars["B"].Value != "2" || vars["B"].Sensitive {
			t.Errorf("B: got value=%q sensitive=%v, want value=%q sensitive=%v", vars["B"].Value, vars["B"].Sensitive, "2", false)
		}
		if vars["C"].Value != "3" || !vars["C"].Sensitive {
			t.Errorf("C: got value=%q sensitive=%v, want value=%q sensitive=%v", vars["C"].Value, vars["C"].Sensitive, "3", true)
		}
		for _, v := range appVer.Config.Variable {
			if v.Source != "manual" {
				t.Errorf("var %s: expected source %q, got %q", v.Key, "manual", v.Source)
			}
		}
	})

	t.Run("CreatesOnlyOneVersion", func(t *testing.T) {
		appName := "test-batch-single-version"
		app := &core_v1alpha.App{}
		appID, err := inmem.Client.Create(ctx, appName, app)
		if err != nil {
			t.Fatalf("failed to create app: %v", err)
		}
		app.ID = appID

		nv1 := &app_v1alpha.NamedValue{}
		nv1.SetKey("X")
		nv1.SetValue("10")
		nv1.SetSensitive(false)

		nv2 := &app_v1alpha.NamedValue{}
		nv2.SetKey("Y")
		nv2.SetValue("20")
		nv2.SetSensitive(false)

		res, err := client.SetEnvVars(ctx, appName, []*app_v1alpha.NamedValue{nv1, nv2}, "")
		if err != nil {
			t.Fatalf("SetEnvVars failed: %v", err)
		}
		firstVersionId := res.VersionId()

		var appCheck core_v1alpha.App
		err = ec.Get(ctx, appName, &appCheck)
		if err != nil {
			t.Fatalf("failed to get app: %v", err)
		}

		var appVer core_v1alpha.AppVersion
		err = ec.GetById(ctx, appCheck.ActiveVersion, &appVer)
		if err != nil {
			t.Fatalf("failed to get app version: %v", err)
		}

		if appVer.Version != firstVersionId {
			t.Errorf("active version %q does not match returned version %q", appVer.Version, firstVersionId)
		}
	})

	t.Run("UpdatesExistingVars", func(t *testing.T) {
		appName := "test-batch-update"
		app := &core_v1alpha.App{}
		appID, err := inmem.Client.Create(ctx, appName, app)
		if err != nil {
			t.Fatalf("failed to create app: %v", err)
		}
		app.ID = appID

		// Set initial vars
		nv1 := &app_v1alpha.NamedValue{}
		nv1.SetKey("KEY1")
		nv1.SetValue("old1")
		nv1.SetSensitive(false)

		nv2 := &app_v1alpha.NamedValue{}
		nv2.SetKey("KEY2")
		nv2.SetValue("old2")
		nv2.SetSensitive(false)

		_, err = client.SetEnvVars(ctx, appName, []*app_v1alpha.NamedValue{nv1, nv2}, "")
		if err != nil {
			t.Fatalf("initial SetEnvVars failed: %v", err)
		}

		// Update KEY1 and add KEY3 in a single batch
		upd1 := &app_v1alpha.NamedValue{}
		upd1.SetKey("KEY1")
		upd1.SetValue("new1")
		upd1.SetSensitive(true)

		upd3 := &app_v1alpha.NamedValue{}
		upd3.SetKey("KEY3")
		upd3.SetValue("val3")
		upd3.SetSensitive(false)

		_, err = client.SetEnvVars(ctx, appName, []*app_v1alpha.NamedValue{upd1, upd3}, "")
		if err != nil {
			t.Fatalf("update SetEnvVars failed: %v", err)
		}

		var appCheck core_v1alpha.App
		err = ec.Get(ctx, appName, &appCheck)
		if err != nil {
			t.Fatalf("failed to get app: %v", err)
		}

		var appVer core_v1alpha.AppVersion
		err = ec.GetById(ctx, appCheck.ActiveVersion, &appVer)
		if err != nil {
			t.Fatalf("failed to get app version: %v", err)
		}

		if len(appVer.Config.Variable) != 3 {
			t.Fatalf("expected 3 env vars, got %d", len(appVer.Config.Variable))
		}

		vars := map[string]core_v1alpha.Variable{}
		for _, v := range appVer.Config.Variable {
			vars[v.Key] = v
		}

		if vars["KEY1"].Value != "new1" || !vars["KEY1"].Sensitive {
			t.Errorf("KEY1: got value=%q sensitive=%v, want value=%q sensitive=%v", vars["KEY1"].Value, vars["KEY1"].Sensitive, "new1", true)
		}
		if vars["KEY2"].Value != "old2" {
			t.Errorf("KEY2: got value=%q, want %q (should be unchanged)", vars["KEY2"].Value, "old2")
		}
		if vars["KEY3"].Value != "val3" {
			t.Errorf("KEY3: got value=%q, want %q", vars["KEY3"].Value, "val3")
		}
	})

	t.Run("RejectsMIRENPrefix", func(t *testing.T) {
		appName := "test-batch-miren-reject"
		app := &core_v1alpha.App{}
		appID, err := inmem.Client.Create(ctx, appName, app)
		if err != nil {
			t.Fatalf("failed to create app: %v", err)
		}
		app.ID = appID

		nv1 := &app_v1alpha.NamedValue{}
		nv1.SetKey("GOOD_VAR")
		nv1.SetValue("ok")
		nv1.SetSensitive(false)

		nv2 := &app_v1alpha.NamedValue{}
		nv2.SetKey("MIREN_SECRET")
		nv2.SetValue("bad")
		nv2.SetSensitive(false)

		_, err = client.SetEnvVars(ctx, appName, []*app_v1alpha.NamedValue{nv1, nv2}, "")
		if err == nil {
			t.Fatal("expected error for MIREN_ prefix, got nil")
		}
	})

	t.Run("PerServiceEnvVars", func(t *testing.T) {
		appName := "test-batch-service"
		app := &core_v1alpha.App{}
		appID, err := inmem.Client.Create(ctx, appName, app)
		if err != nil {
			t.Fatalf("failed to create app: %v", err)
		}
		app.ID = appID

		// Create an initial version with a service defined
		appVer := core_v1alpha.AppVersion{
			App: appID,
			Config: core_v1alpha.Config{
				Services: []core_v1alpha.Services{
					{Name: "web"},
				},
			},
		}
		appVer.Version = appName + "-v0"
		avid, err := inmem.Client.Create(ctx, appVer.Version, &appVer)
		if err != nil {
			t.Fatalf("failed to create initial version: %v", err)
		}

		var appRec core_v1alpha.App
		err = ec.Get(ctx, appName, &appRec)
		if err != nil {
			t.Fatalf("failed to get app: %v", err)
		}
		appRec.ActiveVersion = avid
		err = ec.Update(ctx, &appRec)
		if err != nil {
			t.Fatalf("failed to update app: %v", err)
		}

		nv1 := &app_v1alpha.NamedValue{}
		nv1.SetKey("DB_HOST")
		nv1.SetValue("localhost")
		nv1.SetSensitive(false)

		nv2 := &app_v1alpha.NamedValue{}
		nv2.SetKey("DB_PASS")
		nv2.SetValue("secret")
		nv2.SetSensitive(true)

		_, err = client.SetEnvVars(ctx, appName, []*app_v1alpha.NamedValue{nv1, nv2}, "web")
		if err != nil {
			t.Fatalf("SetEnvVars for service failed: %v", err)
		}

		var appCheck core_v1alpha.App
		err = ec.Get(ctx, appName, &appCheck)
		if err != nil {
			t.Fatalf("failed to get app: %v", err)
		}

		var ver core_v1alpha.AppVersion
		err = ec.GetById(ctx, appCheck.ActiveVersion, &ver)
		if err != nil {
			t.Fatalf("failed to get app version: %v", err)
		}

		if len(ver.Config.Services) != 1 {
			t.Fatalf("expected 1 service, got %d", len(ver.Config.Services))
		}

		svc := ver.Config.Services[0]
		if len(svc.Env) != 2 {
			t.Fatalf("expected 2 service env vars, got %d", len(svc.Env))
		}

		envMap := map[string]core_v1alpha.Env{}
		for _, e := range svc.Env {
			envMap[e.Key] = e
		}

		if envMap["DB_HOST"].Value != "localhost" || envMap["DB_HOST"].Sensitive {
			t.Errorf("DB_HOST: got value=%q sensitive=%v, want value=%q sensitive=%v", envMap["DB_HOST"].Value, envMap["DB_HOST"].Sensitive, "localhost", false)
		}
		if envMap["DB_PASS"].Value != "secret" || !envMap["DB_PASS"].Sensitive {
			t.Errorf("DB_PASS: got value=%q sensitive=%v, want value=%q sensitive=%v", envMap["DB_PASS"].Value, envMap["DB_PASS"].Sensitive, "secret", true)
		}

		if len(ver.Config.Variable) != 0 {
			t.Errorf("expected 0 global env vars, got %d", len(ver.Config.Variable))
		}
	})

	t.Run("ServiceNotFound", func(t *testing.T) {
		appName := "test-batch-nosvc"
		app := &core_v1alpha.App{}
		appID, err := inmem.Client.Create(ctx, appName, app)
		if err != nil {
			t.Fatalf("failed to create app: %v", err)
		}
		app.ID = appID

		appVer := core_v1alpha.AppVersion{App: appID}
		appVer.Version = appName + "-v0"
		avid, err := inmem.Client.Create(ctx, appVer.Version, &appVer)
		if err != nil {
			t.Fatalf("failed to create initial version: %v", err)
		}

		var appRec core_v1alpha.App
		err = ec.Get(ctx, appName, &appRec)
		if err != nil {
			t.Fatalf("failed to get app: %v", err)
		}
		appRec.ActiveVersion = avid
		err = ec.Update(ctx, &appRec)
		if err != nil {
			t.Fatalf("failed to update app: %v", err)
		}

		nv := &app_v1alpha.NamedValue{}
		nv.SetKey("X")
		nv.SetValue("1")
		nv.SetSensitive(false)

		_, err = client.SetEnvVars(ctx, appName, []*app_v1alpha.NamedValue{nv}, "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent service, got nil")
		}
	})
}
