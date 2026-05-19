package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/venkatkrishna07/mkdev/internal/hosts"
)

const hostsComment = "managed by mkdev"

func newHostsHelperCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "hosts-helper",
		Short:  "Privileged hosts-file editor (invoked via sudo)",
		Hidden: true,
	}
	cmd.AddCommand(&cobra.Command{
		Use:  "add <host>",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error { return helperAdd(args[0]) },
	})
	cmd.AddCommand(&cobra.Command{
		Use:  "remove <host>",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error { return helperRemove(args[0]) },
	})
	return cmd
}

func helperAdd(host string) error {
	if !hosts.ValidHostname(host) {
		return fmt.Errorf("invalid hostname %q", host)
	}
	body, err := os.ReadFile(hosts.HostsPath)
	if err != nil {
		return err
	}
	next, changed := hosts.AddEntry(string(body), "127.0.0.1", host, hostsComment)
	if !changed {
		return nil
	}
	return atomicWriteHosts(next)
}

func helperRemove(host string) error {
	if !hosts.ValidHostname(host) {
		return fmt.Errorf("invalid hostname %q", host)
	}
	body, err := os.ReadFile(hosts.HostsPath)
	if err != nil {
		return err
	}
	next, changed := hosts.RemoveEntry(string(body), host)
	if !changed {
		return nil
	}
	return atomicWriteHosts(next)
}

// atomicWriteHosts writes data to hosts.HostsPath via a unique temp file +
// rename so a mid-write crash never leaves /etc/hosts truncated or empty,
// and a hostile process can't pre-create the temp path as a symlink. The
// mode bits of the existing file are preserved across the rename.
func atomicWriteHosts(data string) error {
	info, err := os.Stat(hosts.HostsPath)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("/etc", "hosts.mkdev.*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	cleanup := func() { _ = os.Remove(tmp) }
	if _, err := f.Write([]byte(data)); err != nil {
		_ = f.Close()
		cleanup()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tmp, info.Mode().Perm()); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmp, hosts.HostsPath); err != nil {
		cleanup()
		return err
	}
	return nil
}
