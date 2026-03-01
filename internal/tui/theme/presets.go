package theme

var presetOrder = []string{
	"dark-violet", "dark-blue", "dark-green", "dark-cyan", "dark-orange",
	"light-violet", "light-blue", "light-green", "light-cyan", "light-orange",
}

var presets = map[string]Preset{
	// Dark テーマ
	// 共通ベース: Text=#E4E4E7, Muted=#71717A, Dim=#3F3F46, Error=#EF4444, Warning=#F59E0B, BgHighlight=#27272A
	"dark-violet": {
		ID: "dark-violet", Base: "dark", Accent: "violet", Label: "Violet",
		Palette: Palette{
			Accent: "#7C3AED", AccentDim: "#6D28D9",
			Text: "#E4E4E7", Muted: "#71717A", Dim: "#3F3F46",
			Error: "#EF4444", Warning: "#F59E0B", BgHighlight: "#27272A",
		},
	},
	"dark-blue": {
		ID: "dark-blue", Base: "dark", Accent: "blue", Label: "Blue",
		Palette: Palette{
			Accent: "#3B82F6", AccentDim: "#2563EB",
			Text: "#E4E4E7", Muted: "#71717A", Dim: "#3F3F46",
			Error: "#EF4444", Warning: "#F59E0B", BgHighlight: "#27272A",
		},
	},
	"dark-green": {
		ID: "dark-green", Base: "dark", Accent: "green", Label: "Green",
		Palette: Palette{
			Accent: "#10B981", AccentDim: "#059669",
			Text: "#E4E4E7", Muted: "#71717A", Dim: "#3F3F46",
			Error: "#EF4444", Warning: "#F59E0B", BgHighlight: "#27272A",
		},
	},
	"dark-cyan": {
		ID: "dark-cyan", Base: "dark", Accent: "cyan", Label: "Cyan",
		Palette: Palette{
			Accent: "#06B6D4", AccentDim: "#0891B2",
			Text: "#E4E4E7", Muted: "#71717A", Dim: "#3F3F46",
			Error: "#EF4444", Warning: "#F59E0B", BgHighlight: "#27272A",
		},
	},
	"dark-orange": {
		ID: "dark-orange", Base: "dark", Accent: "orange", Label: "Orange",
		Palette: Palette{
			Accent: "#F97316", AccentDim: "#EA580C",
			Text: "#E4E4E7", Muted: "#71717A", Dim: "#3F3F46",
			Error: "#EF4444", Warning: "#F59E0B", BgHighlight: "#27272A",
		},
	},

	// Light テーマ
	// 共通ベース: Text=#18181B, Muted=#A1A1AA, Dim=#D4D4D8, Error=#DC2626, Warning=#D97706, BgHighlight=#F4F4F5
	"light-violet": {
		ID: "light-violet", Base: "light", Accent: "violet", Label: "Violet",
		Palette: Palette{
			Accent: "#7C3AED", AccentDim: "#6D28D9",
			Text: "#18181B", Muted: "#A1A1AA", Dim: "#D4D4D8",
			Error: "#DC2626", Warning: "#D97706", BgHighlight: "#F4F4F5",
		},
	},
	"light-blue": {
		ID: "light-blue", Base: "light", Accent: "blue", Label: "Blue",
		Palette: Palette{
			Accent: "#2563EB", AccentDim: "#1D4ED8",
			Text: "#18181B", Muted: "#A1A1AA", Dim: "#D4D4D8",
			Error: "#DC2626", Warning: "#D97706", BgHighlight: "#F4F4F5",
		},
	},
	"light-green": {
		ID: "light-green", Base: "light", Accent: "green", Label: "Green",
		Palette: Palette{
			Accent: "#059669", AccentDim: "#047857",
			Text: "#18181B", Muted: "#A1A1AA", Dim: "#D4D4D8",
			Error: "#DC2626", Warning: "#D97706", BgHighlight: "#F4F4F5",
		},
	},
	"light-cyan": {
		ID: "light-cyan", Base: "light", Accent: "cyan", Label: "Cyan",
		Palette: Palette{
			Accent: "#0891B2", AccentDim: "#0E7490",
			Text: "#18181B", Muted: "#A1A1AA", Dim: "#D4D4D8",
			Error: "#DC2626", Warning: "#D97706", BgHighlight: "#F4F4F5",
		},
	},
	"light-orange": {
		ID: "light-orange", Base: "light", Accent: "orange", Label: "Orange",
		Palette: Palette{
			Accent: "#EA580C", AccentDim: "#C2410C",
			Text: "#18181B", Muted: "#A1A1AA", Dim: "#D4D4D8",
			Error: "#DC2626", Warning: "#D97706", BgHighlight: "#F4F4F5",
		},
	},
}
