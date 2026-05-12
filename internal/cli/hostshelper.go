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

// atomicWriteHosts writes data to hosts.HostsPath via a sibling temp file +
// rename so a mid-write crash never leaves /etc/hosts truncated or empty.
// Mode bits of the existing file are preserved across the rename.
func atomicWriteHosts(data string) error {
	info, err := os.Stat(hosts.HostsPath)
	if err != nil {
		return err
	}
	tmp := hosts.HostsPath + ".mkdev.tmp"
	if err := os.WriteFile(tmp, []byte(data), info.Mode().Perm()); err != nil {
		return err
	}
	if err := os.Chmod(tmp, info.Mode().Perm()); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, hosts.HostsPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
