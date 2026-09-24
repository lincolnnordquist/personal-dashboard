package widgets

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDocker struct {
	items    []container.Summary
	started  map[string]string
	listErr  error
	inspects []string
}

func (f *fakeDocker) ContainerList(ctx context.Context, options client.ContainerListOptions) (client.ContainerListResult, error) {
	if !options.All {
		return client.ContainerListResult{}, errors.New("expected All: true to include stopped containers")
	}
	return client.ContainerListResult{Items: f.items}, f.listErr
}

func (f *fakeDocker) ContainerInspect(ctx context.Context, id string, _ client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	f.inspects = append(f.inspects, id)
	started, ok := f.started[id]
	if !ok {
		return client.ContainerInspectResult{}, errors.New("no such container")
	}
	return client.ContainerInspectResult{Container: container.InspectResponse{
		State: &container.State{StartedAt: started},
	}}, nil
}

func TestDockerContainers(t *testing.T) {
	now := time.Date(2026, 9, 23, 18, 0, 0, 0, time.UTC)
	fake := &fakeDocker{
		items: []container.Summary{
			{ID: "aaa", Names: []string{"/old-job"}, Image: "busybox", State: container.StateExited, Status: "Exited (0) 2 days ago"},
			{
				ID: "bbb", Names: []string{"/personal-dashboard-postgres-1"}, Image: "postgres:15",
				State: container.StateRunning, Status: "Up 3 hours (healthy)",
				Health: &container.HealthSummary{Status: container.Healthy},
				Labels: map[string]string{"com.docker.compose.project": "personal-dashboard", "com.docker.compose.service": "postgres"},
			},
			{
				ID: "ccc", Names: []string{"/personal-dashboard-backend-1"}, Image: "personal-dashboard-backend",
				State: container.StateRunning, Status: "Up 3 hours",
				Health: &container.HealthSummary{Status: container.NoHealthcheck},
			},
			{ID: "ddd", Names: []string{"/vanished"}, Image: "nginx", State: container.StateRunning, Status: "Up 1 second"},
		},
		started: map[string]string{
			"bbb": "2026-09-23T14:48:00.123456789Z",
			"ccc": "2026-09-20T10:00:00Z",
		},
	}
	d := &DockerClient{api: fake, now: func() time.Time { return now }}

	got, err := d.Containers(context.Background())
	require.NoError(t, err)

	var names []string
	for _, c := range got {
		names = append(names, c.Name)
	}
	assert.Equal(t, []string{"personal-dashboard-backend-1", "personal-dashboard-postgres-1", "vanished", "old-job"}, names,
		"running containers first, then by name; the leading slash is trimmed")
	assert.ElementsMatch(t, []string{"bbb", "ccc", "ddd"}, fake.inspects, "only running containers are inspected")

	backend, postgres, vanished, old := got[0], got[1], got[2], got[3]

	assert.Equal(t, "running", postgres.Status)
	require.NotNil(t, postgres.Health)
	assert.Equal(t, "healthy", *postgres.Health)
	assert.Equal(t, "3h 11m", postgres.Uptime)
	assert.Equal(t, "personal-dashboard", *postgres.Project)
	assert.Equal(t, "postgres", *postgres.Service)

	assert.Nil(t, backend.Health, `"none" means the container has no healthcheck`)
	assert.Equal(t, "3d 8h", backend.Uptime)
	assert.Nil(t, backend.Project)

	assert.Empty(t, vanished.Uptime, "a failed inspect leaves uptime empty instead of failing the list")

	assert.Equal(t, "exited", old.Status)
	assert.Empty(t, old.Uptime)
	assert.Equal(t, "Exited (0) 2 days ago", old.StatusText)
}

func TestDockerContainersUnavailable(t *testing.T) {
	d := &DockerClient{api: &fakeDocker{listErr: errors.New("permission denied")}, now: time.Now}
	_, err := d.Containers(context.Background())
	assert.ErrorContains(t, err, "docker: permission denied")
}

func TestFormatUptime(t *testing.T) {
	tests := map[time.Duration]string{
		-time.Second:                    "0s",
		45 * time.Second:                "45s",
		59*time.Minute + 59*time.Second: "59m",
		3*time.Hour + 12*time.Minute:    "3h 12m",
		26 * time.Hour:                  "1d 2h",
		40*24*time.Hour + time.Minute:   "40d 0h",
	}
	for d, want := range tests {
		assert.Equal(t, want, FormatUptime(d), d.String())
	}
}
