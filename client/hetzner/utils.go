package hetzner

import (
	"strconv"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// convertServer converts hcloud.Server to our Server type
func convertServer(server *hcloud.Server) *Server {
	var ipAddress string
	if server.PublicNet.IPv4.IP != nil {
		ipAddress = server.PublicNet.IPv4.IP.String()
	}

	var volumeID string
	if len(server.Volumes) > 0 {
		volumeID = strconv.FormatInt(server.Volumes[0].ID, 10)
	}

	return &Server{
		ID:        server.ID,
		Name:      server.Name,
		Status:    string(server.Status),
		IPAddress: ipAddress,
		VolumeID:  volumeID,
		CreatedAt: server.Created,
	}
}

// convertVolume converts hcloud.Volume to our Volume type
func convertVolume(volume *hcloud.Volume) *Volume {
	return &Volume{
		ID:       volume.ID,
		Name:     volume.Name,
		Size:     volume.Size,
		Location: volume.Location.Name,
		Status:   string(volume.Status),
	}
}

// convertSSHKey converts hcloud.SSHKey to our SSHKey type
func convertSSHKey(sshKey *hcloud.SSHKey) *SSHKey {
	return &SSHKey{
		ID:          sshKey.ID,
		Name:        sshKey.Name,
		Fingerprint: sshKey.Fingerprint,
		PublicKey:   sshKey.PublicKey,
	}
}

// parseServerID converts string server ID to int64
func parseServerID(serverID string) int64 {
	id, err := strconv.ParseInt(serverID, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

// parseVolumeID converts string volume ID to int64
func parseVolumeID(volumeID string) int64 {
	id, err := strconv.ParseInt(volumeID, 10, 64)
	if err != nil {
		return 0
	}
	return id
}
