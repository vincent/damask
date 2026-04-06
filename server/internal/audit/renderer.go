package audit

import (
	"encoding/json"
	"fmt"
)

// RenderHumanReadable produces a short English string for a given event, suitable
// for display in a timeline. The actor name is not included — the caller renders
// the actor separately. Falls back to a generic string on unknown types or
// unparseable payloads.
func RenderHumanReadable(eventType string, rawPayload string) string {
	payload := json.RawMessage(rawPayload)

	switch eventType {
	case EventAssetCreated:
		var p AssetCreatedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		if p.Source == "ingress" && p.SourceID != "" {
			return fmt.Sprintf("imported from ingress source %q", p.SourceID)
		}
		return "uploaded"

	case EventAssetRenamed:
		var p AssetRenamedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		return fmt.Sprintf("renamed from %q to %q", p.Before, p.After)

	case EventAssetMoved:
		var p AssetMovedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		if p.AfterProjectID != nil {
			return fmt.Sprintf("moved to project %q", *p.AfterProjectID)
		}
		return "moved"

	case EventAssetTagged:
		var p AssetTaggedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		return fmt.Sprintf("added tag %q", p.Tag)

	case EventAssetUntagged:
		var p AssetUntaggedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		return fmt.Sprintf("removed tag %q", p.Tag)

	case EventAssetFieldSet:
		var p AssetFieldSetPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		return fmt.Sprintf("set %s to %v", p.FieldName, p.After)

	case EventAssetFieldCleared:
		var p AssetFieldClearedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		return fmt.Sprintf("cleared %s", p.FieldName)

	case EventAssetVersionUploaded:
		var p AssetVersionUploadedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		if p.Comment != "" {
			return fmt.Sprintf("uploaded version %d — %q", p.VersionNum, p.Comment)
		}
		return fmt.Sprintf("uploaded version %d", p.VersionNum)

	case EventAssetVersionRestored:
		var p AssetVersionRestoredPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		return fmt.Sprintf("restored version %d (rolled back from v%d)", p.ToVersionNum, p.FromVersionNum)

	case EventAssetVersionDeleted:
		var p AssetVersionDeletedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		return fmt.Sprintf("deleted version %d", p.VersionNum)

	case EventAssetShared:
		var p AssetSharedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		if p.ExpiresAt != nil {
			return fmt.Sprintf("created share link (expires %s)", *p.ExpiresAt)
		}
		return "created share link"

	case EventAssetShareRevoked:
		return "revoked share link"

	case EventAssetDeleted:
		return "deleted"

	case EventAssetDownloaded:
		var p AssetDownloadedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		if p.Via == "share" {
			return "downloaded via share link"
		}
		return "downloaded"

	case EventAssetVariantCreated:
		var p AssetVariantCreatedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		return fmt.Sprintf("requested %s variant", p.Type)

	case EventAssetVariantDownloaded:
		var p AssetVariantDownloadedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		return fmt.Sprintf("downloaded %s variant", p.Type)

	case EventAssetVariantDeleted:
		var p AssetVariantDeletedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		return fmt.Sprintf("deleted %s variant", p.Type)

	case EventProjectCreated:
		var p ProjectCreatedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		return fmt.Sprintf("created project %q", p.Name)

	case EventProjectRenamed:
		var p ProjectRenamedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		return fmt.Sprintf("renamed project from %q to %q", p.Before, p.After)

	case EventProjectFieldSet:
		var p ProjectFieldSetPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		return fmt.Sprintf("set %s to %v", p.FieldName, p.After)

	case EventProjectFieldCleared:
		var p ProjectFieldClearedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			break
		}
		return fmt.Sprintf("cleared %s", p.FieldName)

	case EventProjectDeleted:
		return "deleted project"
	}

	return fmt.Sprintf("performed %s", eventType)
}
