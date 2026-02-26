package diskio

import (
	"context"
	"os"
)

// mockDiskVolumeOps implements DiskVolumeOps for testing
type mockDiskVolumeOps struct {
	createdDirs   []string
	removedDirs   []string
	existingPaths map[string]bool
	createdImages []mockDiskImage
	removedImages []string

	createDirErr   error
	removeDirErr   error
	createImageErr error
	removeImageErr error
}

type mockDiskImage struct {
	path      string
	sizeBytes int64
}

func newMockDiskVolumeOps() *mockDiskVolumeOps {
	return &mockDiskVolumeOps{
		existingPaths: make(map[string]bool),
	}
}

func (m *mockDiskVolumeOps) CreateVolumeDir(path string) error {
	if m.createDirErr != nil {
		return m.createDirErr
	}
	m.createdDirs = append(m.createdDirs, path)
	m.existingPaths[path] = true
	return nil
}

func (m *mockDiskVolumeOps) RemoveVolumeDir(path string) error {
	if m.removeDirErr != nil {
		return m.removeDirErr
	}
	m.removedDirs = append(m.removedDirs, path)
	delete(m.existingPaths, path)
	return nil
}

func (m *mockDiskVolumeOps) VolumePathExists(path string) bool {
	return m.existingPaths[path]
}

func (m *mockDiskVolumeOps) CreateDiskImage(path string, sizeBytes int64) error {
	if m.createImageErr != nil {
		return m.createImageErr
	}
	m.createdImages = append(m.createdImages, mockDiskImage{path: path, sizeBytes: sizeBytes})
	m.existingPaths[path] = true
	return nil
}

func (m *mockDiskVolumeOps) RemoveDiskImage(path string) error {
	if m.removeImageErr != nil {
		return m.removeImageErr
	}
	m.removedImages = append(m.removedImages, path)
	delete(m.existingPaths, path)
	return nil
}

// mockDiskMountOps implements DiskMountOps for testing
type mockDiskMountOps struct {
	createdDirs   []string
	removedFiles  []string
	attachedLoops []string
	detachedLoops []string
	mounts        []diskMockMount
	unmounts      []string
	mountedPaths  map[string]bool
	formattedDevs map[string]string
	formatCalls   []diskMockFormat

	createDirErr  error
	attachErr     error
	detachErr     error
	mountErr      error
	unmountErr    error
	isFormattedFn func(device, filesystem string) (bool, error)
	formatErr     error

	nextLoopDevice string
}

type diskMockMount struct {
	device     string
	mountPath  string
	filesystem string
	readOnly   bool
}

type diskMockFormat struct {
	device     string
	filesystem string
}

func newMockDiskMountOps() *mockDiskMountOps {
	return &mockDiskMountOps{
		mountedPaths:   make(map[string]bool),
		formattedDevs:  make(map[string]string),
		nextLoopDevice: "/dev/loop0",
	}
}

func (m *mockDiskMountOps) CreateDir(path string, _ os.FileMode) error {
	if m.createDirErr != nil {
		return m.createDirErr
	}
	m.createdDirs = append(m.createdDirs, path)
	return nil
}

func (m *mockDiskMountOps) RemoveFile(path string) error {
	m.removedFiles = append(m.removedFiles, path)
	return nil
}

func (m *mockDiskMountOps) LoopAttach(imagePath string) (string, error) {
	if m.attachErr != nil {
		return "", m.attachErr
	}
	m.attachedLoops = append(m.attachedLoops, imagePath)
	return m.nextLoopDevice, nil
}

func (m *mockDiskMountOps) LoopDetach(devicePath string) error {
	if m.detachErr != nil {
		return m.detachErr
	}
	m.detachedLoops = append(m.detachedLoops, devicePath)
	return nil
}

func (m *mockDiskMountOps) Mount(device, mountPath, filesystem string, readOnly bool) error {
	if m.mountErr != nil {
		return m.mountErr
	}
	m.mounts = append(m.mounts, diskMockMount{
		device:     device,
		mountPath:  mountPath,
		filesystem: filesystem,
		readOnly:   readOnly,
	})
	m.mountedPaths[mountPath] = true
	return nil
}

func (m *mockDiskMountOps) Unmount(path string) error {
	if m.unmountErr != nil {
		return m.unmountErr
	}
	m.unmounts = append(m.unmounts, path)
	delete(m.mountedPaths, path)
	return nil
}

func (m *mockDiskMountOps) IsMounted(path string) bool {
	return m.mountedPaths[path]
}

func (m *mockDiskMountOps) IsFormatted(device, filesystem string) (bool, error) {
	if m.isFormattedFn != nil {
		return m.isFormattedFn(device, filesystem)
	}
	if fs, ok := m.formattedDevs[device]; ok {
		return fs == filesystem, nil
	}
	return false, nil
}

func (m *mockDiskMountOps) FormatDevice(_ context.Context, device, filesystem string) error {
	if m.formatErr != nil {
		return m.formatErr
	}
	m.formatCalls = append(m.formatCalls, diskMockFormat{device: device, filesystem: filesystem})
	m.formattedDevs[device] = filesystem
	return nil
}
