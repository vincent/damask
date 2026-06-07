package workflow

import (
	_ "embed"
	"encoding/json"
	"sort"
)

type Template struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TriggerType string `json:"trigger_type"`
	Graph       string `json:"graph"`
}

var (
	//go:embed templates/blank_manual.json
	templateBlankManual string
	//go:embed templates/image_resize_on_upload.json
	templateImageResizeOnUpload string
	//go:embed templates/share_on_upload.json
	templateShareOnUpload string
	//go:embed templates/tag_video_uploads.json
	templateTagVideoUploads string
	//go:embed templates/move_pdfs_to_folder.json
	templateMovePDFsToFolder string
)

func Templates() []Template {
	templates := []Template{
		newTemplate("blank-manual", "Start Blank", "Manual trigger with a clean canvas.", templateBlankManual),
		newTemplate(
			"image-resize-on-upload",
			"Resize Images On Upload",
			"Resize new image uploads into a web-friendly variant.",
			templateImageResizeOnUpload,
		),
		newTemplate(
			"share-on-upload",
			"Create Share On Upload",
			"Generate a share link every time a new asset lands in the library.",
			templateShareOnUpload,
		),
		newTemplate(
			"tag-video-uploads",
			"Tag Video Uploads",
			"Automatically tag video uploads for downstream review.",
			templateTagVideoUploads,
		),
		newTemplate(
			"move-pdfs-to-folder",
			"Route PDFs To Folder",
			"Move new PDFs into a chosen folder for triage.",
			templateMovePDFsToFolder,
		),
	}
	sort.Slice(templates, func(i, j int) bool { return templates[i].Name < templates[j].Name })
	return templates
}

const triggerTypeManual = "trigger.manual"

func newTemplate(id, name, description, graph string) Template {
	triggerType := triggerTypeManual
	var g Graph
	if err := json.Unmarshal([]byte(graph), &g); err == nil {
		if trigger, triggerErr := g.TriggerNode(); triggerErr == nil {
			triggerType = trigger.Type
		}
	}
	return Template{
		ID:          id,
		Name:        name,
		Description: description,
		TriggerType: triggerType,
		Graph:       graph,
	}
}
