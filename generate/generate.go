// Package generate implements functions generating container config files.
package generate

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate/seccomp"
	"github.com/syndtr/gocapability/capability"
)

var (
	// Namespaces include the names of supported namespaces.
	Namespaces = []string{"network", "pid", "mount", "ipc", "uts", "user", "cgroup"}
)

// Generator represents a generator for a container spec.
type Generator struct {
	spec         *rspec.Spec
	HostSpecific bool
}

// ExportOptions have toggles for exporting only certain parts of the specification
type ExportOptions struct {
	Seccomp bool // seccomp toggles if only seccomp should be exported
}

// New creates a spec Generator with the default spec.
func New() Generator {
	spec := rspec.Spec{
		Version: rspec.Version,
		Platform: rspec.Platform{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
		Root: rspec.Root{
			Path:     "",
			Readonly: false,
		},
		Process: rspec.Process{
			Terminal: false,
			User:     rspec.User{},
			Args: []string{
				"sh",
			},
			Env: []string{
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"TERM=xterm",
			},
			Cwd: "/",
			Capabilities: []string{
				"CAP_CHOWN",
				"CAP_DAC_OVERRIDE",
				"CAP_FSETID",
				"CAP_FOWNER",
				"CAP_MKNOD",
				"CAP_NET_RAW",
				"CAP_SETGID",
				"CAP_SETUID",
				"CAP_SETFCAP",
				"CAP_SETPCAP",
				"CAP_NET_BIND_SERVICE",
				"CAP_SYS_CHROOT",
				"CAP_KILL",
				"CAP_AUDIT_WRITE",
			},
			Rlimits: []rspec.Rlimit{
				{
					Type: "RLIMIT_NOFILE",
					Hard: uint64(1024),
					Soft: uint64(1024),
				},
			},
		},
		Hostname: "mrsdalloway",
		Mounts: []rspec.Mount{
			{
				Destination: "/proc",
				Type:        "proc",
				Source:      "proc",
				Options:     nil,
			},
			{
				Destination: "/dev",
				Type:        "tmpfs",
				Source:      "tmpfs",
				Options:     []string{"nosuid", "strictatime", "mode=755", "size=65536k"},
			},
			{
				Destination: "/dev/pts",
				Type:        "devpts",
				Source:      "devpts",
				Options:     []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620", "gid=5"},
			},
			{
				Destination: "/dev/shm",
				Type:        "tmpfs",
				Source:      "shm",
				Options:     []string{"nosuid", "noexec", "nodev", "mode=1777", "size=65536k"},
			},
			{
				Destination: "/dev/mqueue",
				Type:        "mqueue",
				Source:      "mqueue",
				Options:     []string{"nosuid", "noexec", "nodev"},
			},
			{
				Destination: "/sys",
				Type:        "sysfs",
				Source:      "sysfs",
				Options:     []string{"nosuid", "noexec", "nodev", "ro"},
			},
		},
		Linux: &rspec.Linux{
			Resources: &rspec.Resources{
				Devices: []rspec.DeviceCgroup{
					{
						Allow:  false,
						Access: strPtr("rwm"),
					},
				},
			},
			Namespaces: []rspec.Namespace{
				{
					Type: "pid",
				},
				{
					Type: "network",
				},
				{
					Type: "ipc",
				},
				{
					Type: "uts",
				},
				{
					Type: "mount",
				},
			},
			Devices: []rspec.Device{},
		},
	}
	spec.Linux.Seccomp = seccomp.DefaultProfile(&spec)
	return Generator{
		spec: &spec,
	}
}

// NewFromSpec creates a spec Generator from a given spec.
func NewFromSpec(spec *rspec.Spec) Generator {
	return Generator{
		spec: spec,
	}
}

// NewFromFile loads the template specifed in a file into a spec Generator.
func NewFromFile(path string) (Generator, error) {
	cf, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Generator{}, fmt.Errorf("template configuration at %s not found", path)
		}
	}
	defer cf.Close()

	return NewFromTemplate(cf)
}

// NewFromTemplate loads the template from io.Reader into a spec Generator.
func NewFromTemplate(r io.Reader) (Generator, error) {
	var spec rspec.Spec
	if err := json.NewDecoder(r).Decode(&spec); err != nil {
		return Generator{}, err
	}
	return Generator{
		spec: &spec,
	}, nil
}

// SetSpec sets the spec in the Generator g.
func (g *Generator) SetSpec(spec *rspec.Spec) {
	g.spec = spec
}

// Spec gets the spec in the Generator g.
func (g *Generator) Spec() *rspec.Spec {
	return g.spec
}

// Save writes the spec into w.
func (g *Generator) Save(w io.Writer, exportOpts ExportOptions) (err error) {
	var data []byte

	if exportOpts.Seccomp {
		data, err = json.MarshalIndent(g.spec.Linux.Seccomp, "", "\t")
	} else {
		data, err = json.MarshalIndent(g.spec, "", "\t")
	}
	if err != nil {
		return err
	}

	_, err = w.Write(data)
	if err != nil {
		return err
	}

	return nil
}

// SaveToFile writes the spec into a file.
func (g *Generator) SaveToFile(path string, exportOpts ExportOptions) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return g.Save(f, exportOpts)
}

// SetVersion sets g.spec.Version.
func (g *Generator) SetVersion(version string) {
	g.initSpec()
	g.spec.Version = version
}

// SetRootPath sets g.spec.Root.Path.
func (g *Generator) SetRootPath(path string) {
	g.initSpec()
	g.spec.Root.Path = path
}

// SetRootReadonly sets g.spec.Root.Readonly.
func (g *Generator) SetRootReadonly(b bool) {
	g.initSpec()
	g.spec.Root.Readonly = b
}

// SetHostname sets g.spec.Hostname.
func (g *Generator) SetHostname(s string) {
	g.initSpec()
	g.spec.Hostname = s
}

// ClearAnnotations clears g.spec.Annotations.
func (g *Generator) ClearAnnotations() {
	if g.spec == nil {
		return
	}
	g.spec.Annotations = make(map[string]string)
}

// AddAnnotation adds an annotation into g.spec.Annotations.
func (g *Generator) AddAnnotation(key, value string) {
	g.initSpecAnnotations()
	g.spec.Annotations[key] = value
}

// RemoveAnnotation remove an annotation from g.spec.Annotations.
func (g *Generator) RemoveAnnotation(key string) {
	if g.spec == nil || g.spec.Annotations == nil {
		return
	}
	delete(g.spec.Annotations, key)
}

// SetPlatformOS sets g.spec.Process.OS.
func (g *Generator) SetPlatformOS(os string) {
	g.initSpec()
	g.spec.Platform.OS = os
}

// SetPlatformArch sets g.spec.Platform.Arch.
func (g *Generator) SetPlatformArch(arch string) {
	g.initSpec()
	g.spec.Platform.Arch = arch
}

// SetProcessUID sets g.spec.Process.User.UID.
func (g *Generator) SetProcessUID(uid uint32) {
	g.initSpec()
	g.spec.Process.User.UID = uid
}

// SetProcessGID sets g.spec.Process.User.GID.
func (g *Generator) SetProcessGID(gid uint32) {
	g.initSpec()
	g.spec.Process.User.GID = gid
}

// SetProcessCwd sets g.spec.Process.Cwd.
func (g *Generator) SetProcessCwd(cwd string) {
	g.initSpec()
	g.spec.Process.Cwd = cwd
}

// SetProcessNoNewPrivileges sets g.spec.Process.NoNewPrivileges.
func (g *Generator) SetProcessNoNewPrivileges(b bool) {
	g.initSpec()
	g.spec.Process.NoNewPrivileges = b
}

// SetProcessTerminal sets g.spec.Process.Terminal.
func (g *Generator) SetProcessTerminal(b bool) {
	g.initSpec()
	g.spec.Process.Terminal = b
}

// SetProcessApparmorProfile sets g.spec.Process.ApparmorProfile.
func (g *Generator) SetProcessApparmorProfile(prof string) {
	g.initSpec()
	g.spec.Process.ApparmorProfile = prof
}

// SetProcessArgs sets g.spec.Process.Args.
func (g *Generator) SetProcessArgs(args []string) {
	g.initSpec()
	g.spec.Process.Args = args
}

// ClearProcessEnv clears g.spec.Process.Env.
func (g *Generator) ClearProcessEnv() {
	if g.spec == nil {
		return
	}
	g.spec.Process.Env = []string{}
}

// AddProcessEnv adds env into g.spec.Process.Env.
func (g *Generator) AddProcessEnv(env string) {
	g.initSpec()
	g.spec.Process.Env = append(g.spec.Process.Env, env)
}

// ClearProcessAdditionalGids clear g.spec.Process.AdditionalGids.
func (g *Generator) ClearProcessAdditionalGids() {
	if g.spec == nil {
		return
	}
	g.spec.Process.User.AdditionalGids = []uint32{}
}

// AddProcessAdditionalGid adds an additional gid into g.spec.Process.AdditionalGids.
func (g *Generator) AddProcessAdditionalGid(gid uint32) {
	g.initSpec()
	for _, group := range g.spec.Process.User.AdditionalGids {
		if group == gid {
			return
		}
	}
	g.spec.Process.User.AdditionalGids = append(g.spec.Process.User.AdditionalGids, gid)
}

// SetProcessSelinuxLabel sets g.spec.Process.SelinuxLabel.
func (g *Generator) SetProcessSelinuxLabel(label string) {
	g.initSpec()
	g.spec.Process.SelinuxLabel = label
}

// SetLinuxCgroupsPath sets g.spec.Linux.CgroupsPath.
func (g *Generator) SetLinuxCgroupsPath(path string) {
	g.initSpecLinux()
	g.spec.Linux.CgroupsPath = strPtr(path)
}

// SetLinuxMountLabel sets g.spec.Linux.MountLabel.
func (g *Generator) SetLinuxMountLabel(label string) {
	g.initSpecLinux()
	g.spec.Linux.MountLabel = label
}

// SetLinuxResourcesDisableOOMKiller sets g.spec.Linux.Resources.DisableOOMKiller.
func (g *Generator) SetLinuxResourcesDisableOOMKiller(disable bool) {
	g.initSpecLinuxResources()
	g.spec.Linux.Resources.DisableOOMKiller = &disable
}

// SetLinuxResourcesOOMScoreAdj sets g.spec.Linux.Resources.OOMScoreAdj.
func (g *Generator) SetLinuxResourcesOOMScoreAdj(adj int) {
	g.initSpecLinuxResources()
	g.spec.Linux.Resources.OOMScoreAdj = &adj
}

// SetLinuxResourcesCPUShares sets g.spec.Linux.Resources.CPU.Shares.
func (g *Generator) SetLinuxResourcesCPUShares(shares uint64) {
	g.initSpecLinuxResourcesCPU()
	g.spec.Linux.Resources.CPU.Shares = &shares
}

// SetLinuxResourcesCPUQuota sets g.spec.Linux.Resources.CPU.Quota.
func (g *Generator) SetLinuxResourcesCPUQuota(quota uint64) {
	g.initSpecLinuxResourcesCPU()
	g.spec.Linux.Resources.CPU.Quota = &quota
}

// SetLinuxResourcesCPUPeriod sets g.spec.Linux.Resources.CPU.Period.
func (g *Generator) SetLinuxResourcesCPUPeriod(period uint64) {
	g.initSpecLinuxResourcesCPU()
	g.spec.Linux.Resources.CPU.Period = &period
}

// SetLinuxResourcesCPURealtimeRuntime sets g.spec.Linux.Resources.CPU.RealtimeRuntime.
func (g *Generator) SetLinuxResourcesCPURealtimeRuntime(time uint64) {
	g.initSpecLinuxResourcesCPU()
	g.spec.Linux.Resources.CPU.RealtimeRuntime = &time
}

// SetLinuxResourcesCPURealtimePeriod sets g.spec.Linux.Resources.CPU.RealtimePeriod.
func (g *Generator) SetLinuxResourcesCPURealtimePeriod(period uint64) {
	g.initSpecLinuxResourcesCPU()
	g.spec.Linux.Resources.CPU.RealtimePeriod = &period
}

// SetLinuxResourcesCPUCpus sets g.spec.Linux.Resources.CPU.Cpus.
func (g *Generator) SetLinuxResourcesCPUCpus(cpus string) {
	g.initSpecLinuxResourcesCPU()
	g.spec.Linux.Resources.CPU.Cpus = &cpus
}

// SetLinuxResourcesCPUMems sets g.spec.Linux.Resources.CPU.Mems.
func (g *Generator) SetLinuxResourcesCPUMems(mems string) {
	g.initSpecLinuxResourcesCPU()
	g.spec.Linux.Resources.CPU.Mems = &mems
}

// SetLinuxResourcesMemoryLimit sets g.spec.Linux.Resources.Memory.Limit.
func (g *Generator) SetLinuxResourcesMemoryLimit(limit uint64) {
	g.initSpecLinuxResourcesMemory()
	g.spec.Linux.Resources.Memory.Limit = &limit
}

// SetLinuxResourcesMemoryReservation sets g.spec.Linux.Resources.Memory.Reservation.
func (g *Generator) SetLinuxResourcesMemoryReservation(reservation uint64) {
	g.initSpecLinuxResourcesMemory()
	g.spec.Linux.Resources.Memory.Reservation = &reservation
}

// SetLinuxResourcesMemorySwap sets g.spec.Linux.Resources.Memory.Swap.
func (g *Generator) SetLinuxResourcesMemorySwap(swap uint64) {
	g.initSpecLinuxResourcesMemory()
	g.spec.Linux.Resources.Memory.Swap = &swap
}

// SetLinuxResourcesMemoryKernel sets g.spec.Linux.Resources.Memory.Kernel.
func (g *Generator) SetLinuxResourcesMemoryKernel(kernel uint64) {
	g.initSpecLinuxResourcesMemory()
	g.spec.Linux.Resources.Memory.Kernel = &kernel
}

// SetLinuxResourcesMemoryKernelTCP sets g.spec.Linux.Resources.Memory.KernelTCP.
func (g *Generator) SetLinuxResourcesMemoryKernelTCP(kernelTCP uint64) {
	g.initSpecLinuxResourcesMemory()
	g.spec.Linux.Resources.Memory.KernelTCP = &kernelTCP
}

// SetLinuxResourcesMemorySwappiness sets g.spec.Linux.Resources.Memory.Swappiness.
func (g *Generator) SetLinuxResourcesMemorySwappiness(swappiness uint64) {
	g.initSpecLinuxResourcesMemory()
	g.spec.Linux.Resources.Memory.Swappiness = &swappiness
}

// SetLinuxResourcesPidsLimit sets g.spec.Linux.Resources.Pids.Limit.
func (g *Generator) SetLinuxResourcesPidsLimit(limit int64) {
	g.initSpecLinuxResourcesPids()
	g.spec.Linux.Resources.Pids.Limit = &limit
}

// ClearLinuxSysctl clears g.spec.Linux.Sysctl.
func (g *Generator) ClearLinuxSysctl() {
	if g.spec == nil || g.spec.Linux == nil {
		return
	}
	g.spec.Linux.Sysctl = make(map[string]string)
}

// AddLinuxSysctl adds a new sysctl config into g.spec.Linux.Sysctl.
func (g *Generator) AddLinuxSysctl(key, value string) {
	g.initSpecLinuxSysctl()
	g.spec.Linux.Sysctl[key] = value
}

// RemoveLinuxSysctl removes a sysctl config from g.spec.Linux.Sysctl.
func (g *Generator) RemoveLinuxSysctl(key string) {
	if g.spec == nil || g.spec.Linux == nil || g.spec.Linux.Sysctl == nil {
		return
	}
	delete(g.spec.Linux.Sysctl, key)
}

// ClearLinuxUIDMappings clear g.spec.Linux.UIDMappings.
func (g *Generator) ClearLinuxUIDMappings() {
	if g.spec == nil || g.spec.Linux == nil {
		return
	}
	g.spec.Linux.UIDMappings = []rspec.IDMapping{}
}

// AddLinuxUIDMapping adds uidMap into g.spec.Linux.UIDMappings.
func (g *Generator) AddLinuxUIDMapping(hid, cid, size uint32) {
	idMapping := rspec.IDMapping{
		HostID:      hid,
		ContainerID: cid,
		Size:        size,
	}

	g.initSpecLinux()
	g.spec.Linux.UIDMappings = append(g.spec.Linux.UIDMappings, idMapping)
}

// ClearLinuxGIDMappings clear g.spec.Linux.GIDMappings.
func (g *Generator) ClearLinuxGIDMappings() {
	if g.spec == nil || g.spec.Linux == nil {
		return
	}
	g.spec.Linux.GIDMappings = []rspec.IDMapping{}
}

// AddLinuxGIDMapping adds gidMap into g.spec.Linux.GIDMappings.
func (g *Generator) AddLinuxGIDMapping(hid, cid, size uint32) {
	idMapping := rspec.IDMapping{
		HostID:      hid,
		ContainerID: cid,
		Size:        size,
	}

	g.initSpecLinux()
	g.spec.Linux.GIDMappings = append(g.spec.Linux.GIDMappings, idMapping)
}

// SetLinuxRootPropagation sets g.spec.Linux.RootfsPropagation.
func (g *Generator) SetLinuxRootPropagation(rp string) error {
	switch rp {
	case "":
	case "private":
	case "rprivate":
	case "slave":
	case "rslave":
	case "shared":
	case "rshared":
	default:
		return fmt.Errorf("rootfs-propagation must be empty or one of private|rprivate|slave|rslave|shared|rshared")
	}
	g.initSpecLinux()
	g.spec.Linux.RootfsPropagation = rp
	return nil
}

// ClearPreStartHooks clear g.spec.Hooks.Prestart.
func (g *Generator) ClearPreStartHooks() {
	if g.spec == nil {
		return
	}
	g.spec.Hooks.Prestart = []rspec.Hook{}
}

// AddPreStartHook add a prestart hook into g.spec.Hooks.Prestart.
func (g *Generator) AddPreStartHook(path string, args []string) {
	g.initSpec()
	hook := rspec.Hook{Path: path, Args: args}
	g.spec.Hooks.Prestart = append(g.spec.Hooks.Prestart, hook)
}

// ClearPostStopHooks clear g.spec.Hooks.Poststop.
func (g *Generator) ClearPostStopHooks() {
	if g.spec == nil {
		return
	}
	g.spec.Hooks.Poststop = []rspec.Hook{}
}

// AddPostStopHook adds a poststop hook into g.spec.Hooks.Poststop.
func (g *Generator) AddPostStopHook(path string, args []string) {
	g.initSpec()
	hook := rspec.Hook{Path: path, Args: args}
	g.spec.Hooks.Poststop = append(g.spec.Hooks.Poststop, hook)
}

// ClearPostStartHooks clear g.spec.Hooks.Poststart.
func (g *Generator) ClearPostStartHooks() {
	if g.spec == nil {
		return
	}
	g.spec.Hooks.Poststart = []rspec.Hook{}
}

// AddPostStartHook adds a poststart hook into g.spec.Hooks.Poststart.
func (g *Generator) AddPostStartHook(path string, args []string) {
	g.initSpec()
	hook := rspec.Hook{Path: path, Args: args}
	g.spec.Hooks.Poststart = append(g.spec.Hooks.Poststart, hook)
}

// AddTmpfsMount adds a tmpfs mount into g.spec.Mounts.
func (g *Generator) AddTmpfsMount(dest string, options []string) {
	mnt := rspec.Mount{
		Destination: dest,
		Type:        "tmpfs",
		Source:      "tmpfs",
		Options:     options,
	}

	g.initSpec()
	g.spec.Mounts = append(g.spec.Mounts, mnt)
}

// AddCgroupsMount adds a cgroup mount into g.spec.Mounts.
func (g *Generator) AddCgroupsMount(mountCgroupOption string) error {
	switch mountCgroupOption {
	case "ro":
	case "rw":
		break
	case "no":
		return nil
	default:
		return fmt.Errorf("--mount-cgroups should be one of (ro,rw,no)")
	}

	mnt := rspec.Mount{
		Destination: "/sys/fs/cgroup",
		Type:        "cgroup",
		Source:      "cgroup",
		Options:     []string{"nosuid", "noexec", "nodev", "relatime", mountCgroupOption},
	}
	g.initSpec()
	g.spec.Mounts = append(g.spec.Mounts, mnt)

	return nil
}

// AddBindMount adds a bind mount into g.spec.Mounts.
func (g *Generator) AddBindMount(source, dest, options string) {
	if options == "" {
		options = "ro"
	}

	defaultOptions := []string{"bind"}

	mnt := rspec.Mount{
		Destination: dest,
		Type:        "bind",
		Source:      source,
		Options:     append(defaultOptions, options),
	}
	g.initSpec()
	g.spec.Mounts = append(g.spec.Mounts, mnt)
}

// SetupPrivileged sets up the priviledge-related fields inside g.spec.
func (g *Generator) SetupPrivileged(privileged bool) {
	if privileged {
		// Add all capabilities in privileged mode.
		var finalCapList []string
		for _, cap := range capability.List() {
			if g.HostSpecific && cap > lastCap() {
				continue
			}
			finalCapList = append(finalCapList, fmt.Sprintf("CAP_%s", strings.ToUpper(cap.String())))
		}
		g.initSpecLinux()
		g.spec.Process.Capabilities = finalCapList
		g.spec.Process.SelinuxLabel = ""
		g.spec.Process.ApparmorProfile = ""
		g.spec.Linux.Seccomp = nil
	}
}

func lastCap() capability.Cap {
	last := capability.CAP_LAST_CAP
	// hack for RHEL6 which has no /proc/sys/kernel/cap_last_cap
	if last == capability.Cap(63) {
		last = capability.CAP_BLOCK_SUSPEND
	}

	return last
}

func checkCap(c string, hostSpecific bool) error {
	isValid := false
	cp := strings.ToUpper(c)

	for _, cap := range capability.List() {
		if cp == strings.ToUpper(cap.String()) {
			if hostSpecific && cap > lastCap() {
				return fmt.Errorf("CAP_%s is not supported on the current host", cp)
			}
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("Invalid value passed for adding capability")
	}
	return nil
}

// ClearProcessCapabilities clear g.spec.Process.Capabilities.
func (g *Generator) ClearProcessCapabilities() {
	if g.spec == nil {
		return
	}
	g.spec.Process.Capabilities = []string{}
}

// AddProcessCapability adds a process capability into g.spec.Process.Capabilities.
func (g *Generator) AddProcessCapability(c string) error {
	if err := checkCap(c, g.HostSpecific); err != nil {
		return err
	}

	cp := fmt.Sprintf("CAP_%s", strings.ToUpper(c))

	g.initSpec()
	for _, cap := range g.spec.Process.Capabilities {
		if strings.ToUpper(cap) == cp {
			return nil
		}
	}

	g.spec.Process.Capabilities = append(g.spec.Process.Capabilities, cp)
	return nil
}

// DropProcessCapability drops a process capability from g.spec.Process.Capabilities.
func (g *Generator) DropProcessCapability(c string) error {
	if err := checkCap(c, g.HostSpecific); err != nil {
		return err
	}

	cp := fmt.Sprintf("CAP_%s", strings.ToUpper(c))

	g.initSpec()
	for i, cap := range g.spec.Process.Capabilities {
		if strings.ToUpper(cap) == cp {
			g.spec.Process.Capabilities = append(g.spec.Process.Capabilities[:i], g.spec.Process.Capabilities[i+1:]...)
			return nil
		}
	}

	return nil
}

func mapStrToNamespace(ns string, path string) (rspec.Namespace, error) {
	switch ns {
	case "network":
		return rspec.Namespace{Type: rspec.NetworkNamespace, Path: path}, nil
	case "pid":
		return rspec.Namespace{Type: rspec.PIDNamespace, Path: path}, nil
	case "mount":
		return rspec.Namespace{Type: rspec.MountNamespace, Path: path}, nil
	case "ipc":
		return rspec.Namespace{Type: rspec.IPCNamespace, Path: path}, nil
	case "uts":
		return rspec.Namespace{Type: rspec.UTSNamespace, Path: path}, nil
	case "user":
		return rspec.Namespace{Type: rspec.UserNamespace, Path: path}, nil
	case "cgroup":
		return rspec.Namespace{Type: rspec.CgroupNamespace, Path: path}, nil
	default:
		return rspec.Namespace{}, fmt.Errorf("Should not reach here!")
	}
}

// ClearLinuxNamespaces clear g.spec.Linux.Namespaces.
func (g *Generator) ClearLinuxNamespaces() {
	if g.spec == nil || g.spec.Linux == nil {
		return
	}
	g.spec.Linux.Namespaces = []rspec.Namespace{}
}

// AddOrReplaceLinuxNamespace adds or replaces a namespace inside
// g.spec.Linux.Namespaces.
func (g *Generator) AddOrReplaceLinuxNamespace(ns string, path string) error {
	namespace, err := mapStrToNamespace(ns, path)
	if err != nil {
		return err
	}

	g.initSpecLinux()
	for i, ns := range g.spec.Linux.Namespaces {
		if ns.Type == namespace.Type {
			g.spec.Linux.Namespaces[i] = namespace
			return nil
		}
	}
	g.spec.Linux.Namespaces = append(g.spec.Linux.Namespaces, namespace)
	return nil
}

// RemoveLinuxNamespace removes a namespace from g.spec.Linux.Namespaces.
func (g *Generator) RemoveLinuxNamespace(ns string) error {
	namespace, err := mapStrToNamespace(ns, "")
	if err != nil {
		return err
	}

	if g.spec == nil || g.spec.Linux == nil {
		return nil
	}
	for i, ns := range g.spec.Linux.Namespaces {
		if ns.Type == namespace.Type {
			g.spec.Linux.Namespaces = append(g.spec.Linux.Namespaces[:i], g.spec.Linux.Namespaces[i+1:]...)
			return nil
		}
	}
	return nil
}

// strPtr returns the pointer pointing to the string s.
func strPtr(s string) *string { return &s }

// SetSyscallAction adds rules for syscalls with the specified action
func (g *Generator) SetSyscallAction(arguments seccomp.SyscallOpts) error {
	g.initSpecLinuxSeccomp()
	return seccomp.ParseSyscallFlag(arguments, g.spec.Linux.Seccomp)
}

// SetDefaultSeccompAction sets the default action for all syscalls not defined
// and then removes any syscall rules with this action already specified.
func (g *Generator) SetDefaultSeccompAction(action string) error {
	g.initSpecLinuxSeccomp()
	return seccomp.ParseDefaultAction(action, g.spec.Linux.Seccomp)
}

// SetDefaultSeccompActionForce only sets the default action for all syscalls not defined
func (g *Generator) SetDefaultSeccompActionForce(action string) error {
	g.initSpecLinuxSeccomp()
	return seccomp.ParseDefaultActionForce(action, g.spec.Linux.Seccomp)
}

// SetSeccompArchitecture sets the supported seccomp architectures
func (g *Generator) SetSeccompArchitecture(architecture string) error {
	g.initSpecLinuxSeccomp()
	return seccomp.ParseArchitectureFlag(architecture, g.spec.Linux.Seccomp)
}

// RemoveSeccompRule removes rules for any specified syscalls
func (g *Generator) RemoveSeccompRule(arguments string) error {
	g.initSpecLinuxSeccomp()
	return seccomp.RemoveAction(arguments, g.spec.Linux.Seccomp)
}

// RemoveAllSeccompRules removes all syscall rules
func (g *Generator) RemoveAllSeccompRules() error {
	g.initSpecLinuxSeccomp()
	return seccomp.RemoveAllSeccompRules(g.spec.Linux.Seccomp)
}

// AddLinuxMaskedPaths adds masked paths into g.spec.Linux.MaskedPaths.
func (g *Generator) AddLinuxMaskedPaths(path string) {
	g.initSpecLinux()
	g.spec.Linux.MaskedPaths = append(g.spec.Linux.MaskedPaths, path)
}

// AddLinuxReadonlyPaths adds readonly paths into g.spec.Linux.MaskedPaths.
func (g *Generator) AddLinuxReadonlyPaths(path string) {
	g.initSpecLinux()
	g.spec.Linux.ReadonlyPaths = append(g.spec.Linux.ReadonlyPaths, path)
}
