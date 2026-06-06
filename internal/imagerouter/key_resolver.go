package imagerouter

import (
	"context"
	"strings"

	"damask/server/internal/ingress"
)

type KeySource string

const (
	SourceWorkspace KeySource = "workspace"
	SourceEnv       KeySource = "env"
	SourceNone      KeySource = "none"
)

type KeyStatus struct {
	KeySet bool      `json:"key_set"`
	Source KeySource `json:"source"`
}

type KeyResolver func(ctx context.Context, workspaceID string) (string, KeySource, error)

type workspaceKeyReader interface {
	GetImageRouterKey(ctx context.Context, workspaceID string) (string, error)
}

func NewKeyResolver(repo workspaceKeyReader, appSecret string, envKey string) KeyResolver {
	return func(ctx context.Context, workspaceID string) (string, KeySource, error) {
		return ResolveKey(ctx, workspaceID, repo, appSecret, envKey)
	}
}

func ResolveKey(
	ctx context.Context,
	workspaceID string,
	repo workspaceKeyReader,
	appSecret string,
	envKey string,
) (string, KeySource, error) {
	encKey, err := repo.GetImageRouterKey(ctx, workspaceID)
	if err != nil {
		return "", SourceNone, err
	}
	if strings.TrimSpace(encKey) != "" {
		plain, decErr := ingress.DecryptConfig(appSecret, encKey)
		if decErr != nil {
			return "", SourceNone, decErr
		}
		return string(plain), SourceWorkspace, nil
	}

	envKey = strings.TrimSpace(envKey)
	if envKey != "" {
		return envKey, SourceEnv, nil
	}
	return "", SourceNone, nil
}

func GetKeyStatus(
	ctx context.Context,
	workspaceID string,
	repo workspaceKeyReader,
	appSecret string,
	envKey string,
) (KeyStatus, error) {
	_, source, err := ResolveKey(ctx, workspaceID, repo, appSecret, envKey)
	if err != nil {
		return KeyStatus{}, err
	}
	return KeyStatus{
		KeySet: source != SourceNone,
		Source: source,
	}, nil
}
