package progress

import (
	"fmt"
	"strings"
)

func progressClass(className string) string {
	base := "bg-primary/20 relative h-2 w-full overflow-hidden rounded-full"
	if strings.TrimSpace(className) != "" {
		base += " " + className
	}
	return base
}

func progressMax(maxValue float64) float64 {
	if maxValue <= 0 {
		return 100
	}
	return maxValue
}

func progressValue(value float64, maxValue float64) float64 {
	if value < 0 {
		return 0
	}
	if value > progressMax(maxValue) {
		return progressMax(maxValue)
	}
	return value
}

func progressTransform(value float64, maxValue float64) string {
	m := progressMax(maxValue)
	v := progressValue(value, m)
	return fmt.Sprintf("transform: translateX(-%.6g%%)", 100-(100*v)/m)
}

func progressFloat(value float64) string {
	return fmt.Sprintf("%.6g", value)
}
