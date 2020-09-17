// +build !luminous,!mimic

package admin

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var sampleCloneStatusPending = []byte(`{
  "status": {
    "state": "pending",
    "source": {
      "volume": "cephfs",
      "subvolume": "jurrasic",
      "snapshot": "dinodna",
      "group": "park"
    }
  } 
}`)

var sampleCloneStatusInProg = []byte(`{
  "status": {
    "state": "in-progress",
    "source": {
      "volume": "cephfs",
      "subvolume": "subvol1",
      "snapshot": "snap1"
    }
  }
}`)

func TestParseCloneStatus(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		_, err := parseCloneStatus(nil, "", errors.New("flub"))
		assert.Error(t, err)
		assert.Equal(t, "flub", err.Error())
	})
	t.Run("statusSet", func(t *testing.T) {
		_, err := parseCloneStatus(nil, "unexpected!", nil)
		assert.Error(t, err)
	})
	t.Run("badJSON", func(t *testing.T) {
		_, err := parseCloneStatus([]byte("_XxXxX"), "", nil)
		assert.Error(t, err)
	})
	t.Run("okPending", func(t *testing.T) {
		status, err := parseCloneStatus(sampleCloneStatusPending, "", nil)
		assert.NoError(t, err)
		if assert.NotNil(t, status) {
			assert.EqualValues(t, ClonePending, status.State)
			assert.EqualValues(t, "cephfs", status.Source.Volume)
			assert.EqualValues(t, "jurrasic", status.Source.SubVolume)
			assert.EqualValues(t, "dinodna", status.Source.Snapshot)
			assert.EqualValues(t, "park", status.Source.Group)
		}
	})
	t.Run("okInProg", func(t *testing.T) {
		status, err := parseCloneStatus(sampleCloneStatusInProg, "", nil)
		assert.NoError(t, err)
		if assert.NotNil(t, status) {
			assert.EqualValues(t, CloneInProgress, status.State)
			assert.EqualValues(t, "cephfs", status.Source.Volume)
			assert.EqualValues(t, "subvol1", status.Source.SubVolume)
			assert.EqualValues(t, "snap1", status.Source.Snapshot)
			assert.EqualValues(t, "", status.Source.Group)
		}
	})
}

func TestCloneSubVolumeSnapshot(t *testing.T) {
	fsa := getFSAdmin(t)
	volume := "cephfs"
	group := "Park"
	subname := "Jurrasic"
	snapname := "dinodna0"
	clonename := "babydino"

	err := fsa.CreateSubVolumeGroup(volume, group, nil)
	assert.NoError(t, err)
	defer func() {
		err := fsa.RemoveSubVolumeGroup(volume, group)
		assert.NoError(t, err)
	}()

	svopts := &SubVolumeOptions{
		Mode: 0750,
		Size: 20 * gibiByte,
	}
	err = fsa.CreateSubVolume(volume, group, subname, svopts)
	assert.NoError(t, err)
	defer func() {
		err := fsa.RemoveSubVolume(volume, group, subname)
		assert.NoError(t, err)
	}()

	err = fsa.CreateSubVolumeSnapshot(volume, group, subname, snapname)
	assert.NoError(t, err)
	defer func() {
		err := fsa.RemoveSubVolumeSnapshot(volume, group, subname, snapname)
		assert.NoError(t, err)
	}()

	err = fsa.ProtectSubVolumeSnapshot(volume, group, subname, snapname)
	assert.NoError(t, err)
	defer func() {
		err := fsa.UnprotectSubVolumeSnapshot(volume, group, subname, snapname)
		assert.NoError(t, err)
	}()

	err = fsa.CloneSubVolumeSnapshot(
		volume, group, subname, snapname, clonename,
		&CloneOptions{TargetGroup: group})
	assert.NoError(t, err)
	defer func() {
		err := fsa.RemoveSubVolume(volume, group, clonename)
		assert.NoError(t, err)
	}()

	for done := false; !done; {
		status, err := fsa.CloneStatus(volume, group, clonename)
		assert.NoError(t, err)
		assert.NotNil(t, status)
		switch status.State {
		case ClonePending, CloneInProgress:
		case CloneComplete:
			done = true
		case CloneFailed:
			t.Fatal("clone failed")
		default:
			t.Fatalf("invalid status.State: %q", status.State)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
