package imagerouter

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"
)

type Model struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Provider      string  `json:"provider"`
	PricePerImage float64 `json:"price_per_image"`
}

type modelsResponseV2 []modelResponseV2

type modelResponseV2 struct {
	ID    string `json:"id"`
	Price struct {
		Min     float64 `json:"min"`
		Average float64 `json:"average"`
	} `json:"price"`
}

const (
	FetchModelsTimeout        = 5 * time.Second
	modelBriaRemoveBackground = "bria/remove-background"
)

var hardcodedModelDefs = []Model{
	{ID: "black-forest-labs/FLUX-1-schnell", PricePerImage: 0.0013},       //nolint:mnd // a price
	{ID: "black-forest-labs/FLUX-2-dev", PricePerImage: 0.009},            //nolint:mnd // a price
	{ID: "black-forest-labs/FLUX-2-flex", PricePerImage: 0.06},            //nolint:mnd // a price
	{ID: "black-forest-labs/FLUX-2-klein-4b-base", PricePerImage: 0.0013}, //nolint:mnd // a price
	{ID: "black-forest-labs/FLUX-2-klein-4b:free", PricePerImage: 0},
	{ID: "black-forest-labs/FLUX-2-klein-4b", PricePerImage: 0.0006},      //nolint:mnd // a price
	{ID: "black-forest-labs/FLUX-2-klein-9b-base", PricePerImage: 0.0042}, //nolint:mnd // a price
	{ID: "black-forest-labs/FLUX-2-klein-9b", PricePerImage: 0.0008},      //nolint:mnd // a price
	{ID: "black-forest-labs/FLUX-2-max", PricePerImage: 0.07},             //nolint:mnd // a price
	{ID: "black-forest-labs/FLUX-2-pro", PricePerImage: 0.03},             //nolint:mnd // a price
	{ID: "black-forest-labs/flux-kontext-dev", PricePerImage: 0.0105},     //nolint:mnd // a price
	{ID: "black-forest-labs/flux-kontext-max", PricePerImage: 0.08},       //nolint:mnd // a price
	{ID: "black-forest-labs/flux-kontext-pro", PricePerImage: 0.04},       //nolint:mnd // a price
	{ID: "black-forest-labs/flux-krea-dev", PricePerImage: 0.0098},        //nolint:mnd // a price
	{ID: "bria/blur-background", PricePerImage: 0.04},                     //nolint:mnd // a price
	{ID: "bria/bria-fibo", PricePerImage: 0.04},                           //nolint:mnd // a price
	{ID: "bria/enhance", PricePerImage: 0.04},                             //nolint:mnd // a price
	{ID: "bria/erase-foreground", PricePerImage: 0.04},                    //nolint:mnd // a price
	{ID: "bria/remove-background:free", PricePerImage: 0},
	{ID: modelBriaRemoveBackground, PricePerImage: 0.0006},        //nolint:mnd // a price
	{ID: "bytedance/seededit-3", PricePerImage: 0.03},             //nolint:mnd // a price
	{ID: "bytedance/seedream-3", PricePerImage: 0.03},             //nolint:mnd // a price
	{ID: "bytedance/seedream-4.5", PricePerImage: 0.04},           //nolint:mnd // a price
	{ID: "bytedance/seedream-4", PricePerImage: 0.03},             //nolint:mnd // a price
	{ID: "bytedance/seedream-5.0-lite", PricePerImage: 0.035},     //nolint:mnd // a price
	{ID: "cagliostrolab/animagine-xl-3.0", PricePerImage: 0.0019}, //nolint:mnd // a price
	{ID: "csslc/ccsr-2x", PricePerImage: 0.0083},                  //nolint:mnd // a price
	{ID: "fal/flux-2-dev-turbo", PricePerImage: 0.0084},           //nolint:mnd // a price
	{ID: "google/gemini-2.5-flash:free", PricePerImage: 0},
	{ID: "google/gemini-2.5-flash", PricePerImage: 0.039}, //nolint:mnd // a price
	{ID: "google/gemini-3-pro", PricePerImage: 0.138},     //nolint:mnd // a price
	{ID: "google/nano-banana-2:free", PricePerImage: 0},
	{ID: "google/nano-banana-2", PricePerImage: 0.0689},   //nolint:mnd // a price
	{ID: "HiDream-ai/HiDream-E1-1", PricePerImage: 0.06},  //nolint:mnd // a price
	{ID: "jingyunliang/swinir-2x", PricePerImage: 0.0045}, //nolint:mnd // a price
	{ID: "midjourney/midjourney", PricePerImage: 0.0848},  //nolint:mnd // a price
	{ID: "onomaai/illustrious-xl", PricePerImage: 0.0019}, //nolint:mnd // a price
	{ID: "openai/gpt-image-1-mini", PricePerImage: 0.051}, //nolint:mnd // a price
	{ID: "openai/gpt-image-1.5:free", PricePerImage: 0},
	{ID: "openai/gpt-image-1.5", PricePerImage: 0.133},                    //nolint:mnd // a price
	{ID: "openai/gpt-image-1", PricePerImage: 0.167},                      //nolint:mnd // a price
	{ID: "openai/gpt-image-2", PricePerImage: 0.0414},                     //nolint:mnd // a price
	{ID: "philz1337x/clarity-2x", PricePerImage: 0.0038},                  //nolint:mnd // a price
	{ID: "prunaai/P-Image-1.0", PricePerImage: 0.0044},                    //nolint:mnd // a price
	{ID: "prunaai/P-Image-Upscale", PricePerImage: 0.005},                 //nolint:mnd // a price
	{ID: "purplesmartai/pony-diffusion-v6-xl", PricePerImage: 0.0019},     //nolint:mnd // a price
	{ID: "qwen/qwen-image-2-pro", PricePerImage: 0.075},                   //nolint:mnd // a price
	{ID: "qwen/qwen-image-2", PricePerImage: 0.035},                       //nolint:mnd // a price
	{ID: "qwen/qwen-image-2512", PricePerImage: 0.0064},                   //nolint:mnd // a price
	{ID: "qwen/qwen-image-edit-2511", PricePerImage: 0.0186},              //nolint:mnd // a price
	{ID: "qwen/qwen-image-edit-plus", PricePerImage: 0.0083},              //nolint:mnd // a price
	{ID: "qwen/qwen-image-edit", PricePerImage: 0.0058},                   //nolint:mnd // a price
	{ID: "qwen/qwen-image-layered", PricePerImage: 0.0211},                //nolint:mnd // a price
	{ID: "qwen/qwen-image", PricePerImage: 0.007},                         //nolint:mnd // a price
	{ID: "recraft/recraft-v3", PricePerImage: 0.04},                       //nolint:mnd // a price
	{ID: "recraft/recraft-vectorize", PricePerImage: 0.01},                //nolint:mnd // a price
	{ID: "reve/reve-1", PricePerImage: 0.025},                             //nolint:mnd // a price
	{ID: "SG161222/RealVisXL", PricePerImage: 0.0019},                     //nolint:mnd // a price
	{ID: "sourceful/riverflow-1.1-base", PricePerImage: 0.039},            //nolint:mnd // a price
	{ID: "sourceful/riverflow-1.1-mini", PricePerImage: 0.0303},           //nolint:mnd // a price
	{ID: "sourceful/riverflow-1.1-pro", PricePerImage: 0.077},             //nolint:mnd // a price
	{ID: "sourceful/riverflow-2-preview-fast", PricePerImage: 0.03},       //nolint:mnd // a price
	{ID: "sourceful/riverflow-2-preview-max", PricePerImage: 0.075},       //nolint:mnd // a price
	{ID: "sourceful/riverflow-2-preview-standard", PricePerImage: 0.0351}, //nolint:mnd // a price
	{ID: "stabilityai/latent-2x", PricePerImage: 0.0038},                  //nolint:mnd // a price
	{ID: "stabilityai/sd3", PricePerImage: 0.0019},                        //nolint:mnd // a price
	{ID: "tencent/hunyuan-image-3", PricePerImage: 0.05},                  //nolint:mnd // a price
	{ID: "test/test", PricePerImage: 0},
	{ID: "wan/wan-2.7-image-pro", PricePerImage: 0.075}, //nolint:mnd // a price
	{ID: "wan/wan-2.7-image", PricePerImage: 0.03},      //nolint:mnd // a price
	{ID: "wavespeed/ghibli", PricePerImage: 0.005},      //nolint:mnd // a price
	{ID: "xAI/grok-imagine-image", PricePerImage: 0.02}, //nolint:mnd // a price
}

var HardcodedModels = normalizeAndSortModels(hardcodedModelDefs)

func FetchModels(ctx context.Context, apiKey string) ([]Model, error) {
	if strings.TrimSpace(apiKey) == "" {
		return append([]Model(nil), HardcodedModels...), nil
	}

	httpClient := &http.Client{Timeout: FetchModelsTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsEndpointURL(), nil)
	if err != nil {
		return fallbackModels(ctx, err), nil
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fallbackModels(ctx, err), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fallbackModels(ctx, err), nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fallbackModels(ctx, err), nil
	}

	var parsed modelsResponseV2
	if err = json.Unmarshal(body, &parsed); err != nil {
		return fallbackModels(ctx, err), nil
	}

	models := make([]Model, 0, len(parsed))
	for _, raw := range parsed {
		price := raw.Price.Average
		if price == 0 {
			price = raw.Price.Min
		}
		models = append(models, normalizeModel(Model{
			ID:            raw.ID,
			PricePerImage: price,
		}))
	}
	return normalizeAndSortModels(models), nil
}

func fallbackModels(ctx context.Context, cause error) []Model {
	slog.WarnContext(ctx, "imagerouter models fallback", "error", cause)
	return append([]Model(nil), HardcodedModels...)
}

func modelsEndpointURL() string {
	base := strings.TrimSuffix(strings.TrimRight(apiBaseURL, "/"), "/v1")
	return base + "/v2/models?inputType=image&outputType=image"
}

func normalizeAndSortModels(in []Model) []Model {
	models := make([]Model, 0, len(in))
	for _, model := range in {
		models = append(models, normalizeModel(model))
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].PricePerImage == models[j].PricePerImage {
			return models[i].Name < models[j].Name
		}
		return models[i].PricePerImage < models[j].PricePerImage
	})
	return models
}

func normalizeModel(model Model) Model {
	if model.Provider == "" || model.Name == "" {
		provider, slug, ok := strings.Cut(model.ID, "/")
		if ok {
			if model.Provider == "" {
				model.Provider = provider
			}
			if model.Name == "" {
				model.Name = humanizeModelSlug(slug)
			}
		}
	}
	if model.Name == "" {
		model.Name = model.ID
	}
	if model.Provider == "" {
		model.Provider = model.ID
	}
	return model
}

func humanizeModelSlug(slug string) string {
	slug = strings.NewReplacer("-", " ", "_", " ").Replace(slug)
	parts := strings.Fields(slug)
	for i, part := range parts {
		parts[i] = formatModelToken(part)
	}
	return strings.Join(parts, " ")
}

func formatModelToken(token string) string {
	if token == "" {
		return token
	}
	if isAllUpper(token) {
		return token
	}

	switch strings.ToLower(token) {
	case "bria":
		return "BRIA"
	case "ccsr":
		return "CCSR"
	case "gpt":
		return "GPT"
	case "qwen":
		return "Qwen"
	case "sd":
		return "SD"
	case "sd3":
		return "SD3"
	case "sdxl":
		return "SDXL"
	case "xai":
		return "xAI"
	case "xl":
		return "XL"
	}

	runes := []rune(strings.ToLower(token))
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func isAllUpper(token string) bool {
	hasLetter := false
	for _, r := range token {
		if unicode.IsLetter(r) {
			hasLetter = true
			if unicode.IsLower(r) {
				return false
			}
		}
	}
	return hasLetter
}
