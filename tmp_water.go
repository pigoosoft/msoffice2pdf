package main

import (
	"context"
	"fmt"
	"os"

	"msoffice2pdf/internal/watermark"
)

func main() {
	src := os.Args[1]
	dst := src + ".wmtest.pdf"
	data, err := os.ReadFile(src)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		panic(err)
	}
	font, err := watermark.ResolveFontPath("")
	fmt.Println("font:", font, err)
	err = watermark.Apply(context.Background(), dst, watermark.Options{
		Primary:  "PigooSoft 信息科技有限公司",
		Angle:    -45,
		Density:  "medium",
		Opacity:  0.25,
		Color:    "#808080",
		FontSize: 0,
	})
	fmt.Println("apply:", err)
	fi, _ := os.Stat(dst)
	fmt.Println("out:", dst, "size:", fi.Size())
}
