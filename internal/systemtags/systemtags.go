package systemtags

const (
	GroupName = "system"
	Watermark = "_watermark"
	Template  = "_template"
)

// All returns all known system tag names.
func All() []string {
	return []string{Watermark, Template}
}

// IsSystem reports whether name is a known system tag.
func IsSystem(name string) bool {
	for _, s := range All() {
		if s == name {
			return true
		}
	}
	return false
}
