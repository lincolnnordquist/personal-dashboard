package widgets

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

const DockerWidgetType = "docker"

// DockerContainer is bound to the GraphQL DockerContainer type.
type DockerContainer struct {
	ID         string
	Name       string
	Status     string
	Health     *string
	Image      string
	Uptime     string
	StatusText string
	Project    *string
	Service    *string
}

// dockerAPI is the subset of the Docker SDK client used here, so tests can substitute a fake.
type dockerAPI interface {
	ContainerList(ctx context.Context, options client.ContainerListOptions) (client.ContainerListResult, error)
	ContainerInspect(ctx context.Context, containerID string, options client.ContainerInspectOptions) (client.ContainerInspectResult, error)
}

// DockerClient reads container status from the Docker socket. It never changes containers.
type DockerClient struct {
	api dockerAPI
	now func() time.Time
}

// NewDockerClient connects using DOCKER_HOST, defaulting to /var/run/docker.sock.
// The connection is made lazily, on the first request.
func NewDockerClient() (*DockerClient, error) {
	c, err := client.New(client.FromEnv)
	if err != nil {
		return nil, err
	}
	return &DockerClient{api: c, now: time.Now}, nil
}

// Containers lists every container, running or not. Running containers come first.
// Docker status must always be current, so results are never cached.
func (d *DockerClient) Containers(ctx context.Context) ([]*DockerContainer, error) {
	list, err := d.api.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("docker: %w", err)
	}

	containers := make([]*DockerContainer, 0, len(list.Items))
	for _, s := range list.Items {
		c := &DockerContainer{
			ID:         s.ID,
			Name:       containerName(s),
			Status:     string(s.State),
			Image:      s.Image,
			StatusText: s.Status,
			Project:    label(s.Labels, "com.docker.compose.project"),
			Service:    label(s.Labels, "com.docker.compose.service"),
		}
		if s.Health != nil && s.Health.Status != "" && s.Health.Status != container.NoHealthcheck {
			health := string(s.Health.Status)
			c.Health = &health
		}
		// The list API has no start time, so running containers are inspected for it.
		// A container removed in between just goes without an uptime.
		if s.State == container.StateRunning {
			if res, err := d.api.ContainerInspect(ctx, s.ID, client.ContainerInspectOptions{}); err == nil && res.Container.State != nil {
				if started, err := time.Parse(time.RFC3339Nano, res.Container.State.StartedAt); err == nil {
					c.Uptime = FormatUptime(d.now().Sub(started))
				}
			}
		}
		containers = append(containers, c)
	}

	slices.SortFunc(containers, func(a, b *DockerContainer) int {
		return cmp.Or(
			cmp.Compare(stateRank(a.Status), stateRank(b.Status)),
			cmp.Compare(a.Name, b.Name),
		)
	})
	return containers, nil
}

func containerName(s container.Summary) string {
	if len(s.Names) == 0 {
		return s.ID[:min(12, len(s.ID))]
	}
	return strings.TrimPrefix(s.Names[0], "/")
}

func label(labels map[string]string, key string) *string {
	if v, ok := labels[key]; ok && v != "" {
		return &v
	}
	return nil
}

func stateRank(state string) int {
	switch container.ContainerState(state) {
	case container.StateRunning:
		return 0
	case container.StateRestarting:
		return 1
	case container.StatePaused:
		return 2
	case container.StateCreated:
		return 3
	default:
		return 4
	}
}

// FormatUptime formats a duration with its two largest units, e.g. "2d 4h", "3h 12m", "45s".
func FormatUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", max(0, int(d.Seconds())))
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
}
