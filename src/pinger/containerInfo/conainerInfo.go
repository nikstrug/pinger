package containerinfo

// Импортируем пакеты
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// Структура для базы данных
type DBContainer struct {
	ContainerID string            `json:"containerID"`
	IP          map[string]string `json:"ip"`
	Status      string            `json:"status"`
	CPU         float64           `json:"cpu"`
	Memory      uint64            `json:"memory"`
	Timestamp   time.Time         `json:"timestamp"`
	Datestamp   time.Time         `json:"datestamp"`
}

// Структура для переменных окружения
type Env struct {
	Networks []string
	BackURL  string
}

// Получение IP контейнеров
func getContainerIPs(c types.Container, networkList []string) map[string]string {
	ips := make(map[string]string)
	for _, network := range networkList {
		if net, exists := c.NetworkSettings.Networks[network]; exists {
			ips[network] = net.IPAddress
		}
	}
	return ips
}

// Получение сетей контейнеров
func getContainerNetworks(c types.Container, networkList []string) []string {
	networks := []string{}
	for _, network := range networkList {
		if _, exists := c.NetworkSettings.Networks[network]; exists {
			networks = append(networks, network)
		}
	}
	return networks
}

// Получение контейнеров клиента, находящиеся в network
func getNetworkContainers(cli *client.Client, network string) ([]types.Container, error) {
	filter := filters.NewArgs()
	filter.Add("network", network)

	return cli.ContainerList(
		context.Background(),
		container.ListOptions{
			All:     true,
			Filters: filter,
		},
	)
}

// Получение статуса контейнеров
func getContainerStatus(cli *client.Client, containerID string) (string, error) {
	info, err := cli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return "", err
	}
	return info.State.Status, nil
}

func calculateCPUPercent(stats container.StatsResponse) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	return (cpuDelta / systemDelta) * 1000.0
}

func getContainerMetrics(cli *client.Client, containerID string) (float64, uint64, error) {
	stats, err := cli.ContainerStats(context.Background(), containerID, false)
	if err != nil {
		return 0, 0, err
	}
	defer stats.Body.Close()
	var containerStats container.StatsResponse
	err = json.NewDecoder(stats.Body).Decode(&containerStats)
	if err != nil {
		return 0, 0, err
	}
	return calculateCPUPercent(containerStats), containerStats.MemoryStats.Usage, nil
}

// Собирает информацию о контейнерах и отправляет их на бэк
func CheckContainers(cli *client.Client, env Env) {
	allContainers := make(map[string]types.Container)

	for _, network := range env.Networks {
		containers, err := getNetworkContainers(cli, network)
		if err != nil {
			log.Printf("Error: getting containers in network %s: %v\n", network, err)
			continue
		}
		for _, c := range containers {
			if _, exists := allContainers[c.ID]; !exists {
				allContainers[c.ID] = c
			}
		}
	}

	req := []DBContainer{}

	for _, c := range allContainers {
		status, err := getContainerStatus(cli, c.ID)
		if err != nil {
			log.Printf("Container %s status error: %v\n", c.Names[0], err)
			continue
		}

		containerNetworks := getContainerNetworks(c, env.Networks)
		ips := getContainerIPs(c, env.Networks)
		cpu, memory, err := getContainerMetrics(cli, c.ID)
		if err != nil {
			log.Fatal(err)
		}
		pingTime := time.Now()
		req = append(req, DBContainer{
			ContainerID: c.ID,
			IP:          ips,
			Status:      status,
			CPU:         cpu,
			Memory:      memory,
			Timestamp:   pingTime,
			Datestamp:   pingTime,
		})

		go sendToBack(req, env)

		log.Printf(
			"Container: %-20s Status: %-10s Networks: %-30s IPs: %v\n",
			c.Names[0],
			status,
			strings.Join(containerNetworks, ", "),
			ips,
		)
	}
}

// Отправляет список с информацией о контейнерах на бэк
func sendToBack(req []DBContainer, env Env) {
	b, err := json.Marshal(req)
	if err != nil {
		log.Println(fmt.Errorf("failed to marshall request: %w", err))
		return
	}
	if _, err := http.Post(env.BackURL, "application/json", bytes.NewBuffer(b)); err != nil {
		log.Println(fmt.Errorf("failed to send: %w", err))
		return
	}
}
