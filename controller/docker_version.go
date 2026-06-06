package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

const (
	dockerImageOwner   = "huiyio"
	dockerImageRepo    = "new-api"
	dockerImageTag     = "custom-home-ui"
	dockerRegistryHost = "ghcr.io"
	dockerPackageURL   = "https://github.com/huiyio/new-api/pkgs/container/new-api"
)

var dockerManifestAccept = strings.Join([]string{
	"application/vnd.oci.image.index.v1+json",
	"application/vnd.oci.image.manifest.v1+json",
	"application/vnd.docker.distribution.manifest.list.v2+json",
	"application/vnd.docker.distribution.manifest.v2+json",
}, ", ")

type dockerManifestPlatform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant,omitempty"`
}

type dockerManifestEntry struct {
	MediaType string                 `json:"mediaType"`
	Digest    string                 `json:"digest"`
	Size      int64                  `json:"size"`
	Platform  dockerManifestPlatform `json:"platform"`
}

type dockerManifestList struct {
	SchemaVersion int                   `json:"schemaVersion"`
	MediaType     string                `json:"mediaType"`
	Manifests     []dockerManifestEntry `json:"manifests"`
}

type dockerManifestConfig struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type dockerManifest struct {
	SchemaVersion int                  `json:"schemaVersion"`
	MediaType     string               `json:"mediaType"`
	Config        dockerManifestConfig `json:"config"`
}

type dockerImageConfig struct {
	Config struct {
		Labels map[string]string `json:"Labels"`
	} `json:"config"`
}

type dockerVersionData struct {
	Image           string `json:"image"`
	TrackingTag     string `json:"tracking_tag"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	LatestRevision  string `json:"latest_revision"`
	LatestDigest    string `json:"latest_digest"`
	UpdateAvailable bool   `json:"update_available"`
	PackageURL      string `json:"package_url"`
}

func dockerHTTPClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second}
}

func parseBearerChallenge(header string) map[string]string {
	result := make(map[string]string)
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return result
	}
	body := header[len(prefix):]
	// split params by comma at top-level
	var parts []string
	var current strings.Builder
	inQuote := false
	for i := 0; i < len(body); i++ {
		ch := body[i]
		if ch == '"' {
			inQuote = !inQuote
			current.WriteByte(ch)
			continue
		}
		if ch == ',' && !inQuote {
			parts = append(parts, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(ch)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	for _, kv := range parts {
		kv = strings.TrimSpace(kv)
		idx := strings.Index(kv, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(kv[:idx])
		value := strings.TrimSpace(kv[idx+1:])
		value = strings.Trim(value, "\"")
		result[key] = value
	}
	return result
}

func fetchDockerToken(challenge map[string]string) (string, error) {
	realm := challenge["realm"]
	if realm == "" {
		return "", errors.New("missing realm in registry challenge")
	}
	req, err := http.NewRequest(http.MethodGet, realm, nil)
	if err != nil {
		return "", err
	}
	q := req.URL.Query()
	if service := challenge["service"]; service != "" {
		q.Set("service", service)
	}
	if scope := challenge["scope"]; scope != "" {
		q.Set("scope", scope)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := dockerHTTPClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token endpoint returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var payload struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := common.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.Token != "" {
		return payload.Token, nil
	}
	if payload.AccessToken != "" {
		return payload.AccessToken, nil
	}
	return "", errors.New("registry token response missing token")
}

func dockerRegistryGet(url, accept, token string) ([]byte, *http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := dockerHTTPClient().Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp, err
	}
	return body, resp, nil
}

func dockerRegistryGetWithAuth(url, accept string) ([]byte, string, string, error) {
	body, resp, err := dockerRegistryGet(url, accept, "")
	if err != nil {
		return nil, "", "", err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		challenge := parseBearerChallenge(resp.Header.Get("WWW-Authenticate"))
		token, err := fetchDockerToken(challenge)
		if err != nil {
			return nil, "", "", err
		}
		body, resp, err = dockerRegistryGet(url, accept, token)
		if err != nil {
			return nil, "", "", err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, token, "", fmt.Errorf("registry returned status %d after auth: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		return body, token, strings.TrimSpace(resp.Header.Get("Docker-Content-Digest")), nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", "", fmt.Errorf("registry returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, "", strings.TrimSpace(resp.Header.Get("Docker-Content-Digest")), nil
}

func dockerRegistryGetWithToken(url, accept, token string) ([]byte, error) {
	body, resp, err := dockerRegistryGet(url, accept, token)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		challenge := parseBearerChallenge(resp.Header.Get("WWW-Authenticate"))
		newToken, err := fetchDockerToken(challenge)
		if err != nil {
			return nil, err
		}
		body, resp, err = dockerRegistryGet(url, accept, newToken)
		if err != nil {
			return nil, err
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func selectAmd64Manifest(list *dockerManifestList) *dockerManifestEntry {
	for i := range list.Manifests {
		entry := &list.Manifests[i]
		if entry.Platform.OS == "linux" && entry.Platform.Architecture == "amd64" {
			return entry
		}
	}
	return nil
}

func isManifestList(mediaType string) bool {
	return mediaType == "application/vnd.oci.image.index.v1+json" ||
		mediaType == "application/vnd.docker.distribution.manifest.list.v2+json"
}

func GetDockerVersion(c *gin.Context) {
	imageRef := fmt.Sprintf("%s/%s/%s:%s", dockerRegistryHost, dockerImageOwner, dockerImageRepo, dockerImageTag)
	manifestURL := fmt.Sprintf("https://%s/v2/%s/%s/manifests/%s", dockerRegistryHost, dockerImageOwner, dockerImageRepo, dockerImageTag)

	manifestBody, token, tagDigest, err := dockerRegistryGetWithAuth(manifestURL, dockerManifestAccept)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("查询 Docker manifest 失败: %s", err.Error()),
		})
		return
	}

	// Detect manifest list vs single manifest by mediaType field.
	var manifestProbe struct {
		MediaType string `json:"mediaType"`
	}
	if err := common.Unmarshal(manifestBody, &manifestProbe); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("解析 Docker manifest 失败: %s", err.Error()),
		})
		return
	}

	var (
		manifest       dockerManifest
		platformDigest string
	)

	if isManifestList(manifestProbe.MediaType) {
		var list dockerManifestList
		if err := common.Unmarshal(manifestBody, &list); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("解析 Docker manifest list 失败: %s", err.Error()),
			})
			return
		}
		entry := selectAmd64Manifest(&list)
		if entry == nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "未在 manifest list 中找到 linux/amd64 平台",
			})
			return
		}
		platformDigest = entry.Digest
		entryURL := fmt.Sprintf("https://%s/v2/%s/%s/manifests/%s", dockerRegistryHost, dockerImageOwner, dockerImageRepo, entry.Digest)
		entryBody, err := dockerRegistryGetWithToken(entryURL, dockerManifestAccept, token)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("查询平台 manifest 失败: %s", err.Error()),
			})
			return
		}
		if err := common.Unmarshal(entryBody, &manifest); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("解析平台 manifest 失败: %s", err.Error()),
			})
			return
		}
	} else {
		if err := common.Unmarshal(manifestBody, &manifest); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("解析 Docker manifest 失败: %s", err.Error()),
			})
			return
		}
	}

	if manifest.Config.Digest == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "manifest 中缺少 config digest",
		})
		return
	}

	configURL := fmt.Sprintf("https://%s/v2/%s/%s/blobs/%s", dockerRegistryHost, dockerImageOwner, dockerImageRepo, manifest.Config.Digest)
	configBody, err := dockerRegistryGetWithToken(configURL, "application/vnd.oci.image.config.v1+json, application/vnd.docker.container.image.v1+json, application/json", token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("查询镜像 config 失败: %s", err.Error()),
		})
		return
	}

	var imageConfig dockerImageConfig
	if err := common.Unmarshal(configBody, &imageConfig); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("解析镜像 config 失败: %s", err.Error()),
		})
		return
	}

	revision := strings.TrimSpace(imageConfig.Config.Labels["org.opencontainers.image.revision"])
	if revision == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "镜像 config 缺少 org.opencontainers.image.revision label",
		})
		return
	}

	shortRev := revision
	if len(shortRev) > 8 {
		shortRev = shortRev[:8]
	}
	latestVersion := fmt.Sprintf("%s-%s", dockerImageTag, shortRev)
	current := common.Version

	updateAvailable := true
	if current == latestVersion {
		updateAvailable = false
	} else if shortRev != "" && strings.Contains(current, shortRev) {
		updateAvailable = false
	}

	latestDigest := tagDigest
	if latestDigest == "" {
		latestDigest = platformDigest
	}

	data := dockerVersionData{
		Image:           imageRef,
		TrackingTag:     dockerImageTag,
		CurrentVersion:  current,
		LatestVersion:   latestVersion,
		LatestRevision:  revision,
		LatestDigest:    latestDigest,
		UpdateAvailable: updateAvailable,
		PackageURL:      dockerPackageURL,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}
