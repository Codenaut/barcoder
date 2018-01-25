package barcodes

import (
	"image"
	"io"

	"github.com/codenaut/barcoder/images"
	"github.com/codenaut/barcoder/zpl"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"gopkg.in/urfave/cli.v1"
)

type internal struct {
	config      BarcodeConfig
	xOffset     int
	yOffset     int
	scaleFactor float64
	width       int
	height      int
}

type BarcodeConfig struct {
	Offset []int
	Unit   string
	Size   []int
	Dpi    int
	Dpmm   int

	Image   []ImageFileConfig
	Qr      []BarcodeProperties
	Code128 []BarcodeProperties
}
type ImageFileConfig struct {
	File       string
	Darkness   uint16
	Properties ImageConfig
}
type BarcodeProperties struct {
	Input      int
	Value      string
	Properties ImageConfig
}

type ImageConfig struct {
	Size     []int
	Position []int
	Center   bool
	Top      bool
	Bottom   bool
	Left     bool
	Right    bool
}

func New(config BarcodeConfig) *internal {
	xOffset := 0
	yOffset := 0

	scaleFactor := float64(1.0)
	if config.Dpmm > 0 {
		scaleFactor = float64(config.Dpmm)
	}
	if len(config.Offset) > 1 {
		xOffset = config.Offset[0]
		yOffset = config.Offset[1]
	}
	width := 0
	height := 0
	if len(config.Size) > 1 {
		width = int(float64(config.Size[0]) * scaleFactor)
		height = int(float64(config.Size[1]) * scaleFactor)

	}
	return &internal{config: config,
		scaleFactor: scaleFactor,
		xOffset:     xOffset, yOffset: yOffset,
		width: width, height: height,
	}
}

func scale(factor float64, size []int) (int, int) {
	if len(size) > 1 {
		x := float64(size[0])
		y := float64(size[1])
		return int(factor * x), int(factor * y)
	}

	return 0, 0
}

func (t *internal) Process(output io.Writer, args cli.Args) error {

	zpl.Start(output)
	for _, img := range t.config.Image {
		if i, err := images.OpenPng(img.File); err != nil {
			return err
		} else {
			darknessLimit := img.Darkness
			if darknessLimit == 0 {
				darknessLimit = 0xafff
			}
			flat := images.FlattenImage(i)
			if err := t.placeImage(flat, img.Properties, darknessLimit, output); err != nil {
				return err
			}

		}
	}
	t.processBarcodes(t.config.Qr, args, output)
	t.processBarcodes(t.config.Code128, args, output)
	zpl.End(output)
	return nil
}
func (t *internal) processBarcodes(confs []BarcodeProperties, args cli.Args, output io.Writer) error {
	for _, conf := range confs {
		str := conf.Value
		if str == "" {
			str = args.Get(conf.Input)
		}
		if qrCode, err := qr.Encode(str, qr.M, qr.Auto); err != nil {
			return err
		} else {
			if err := t.placeBarcode(qrCode, conf.Properties, 0xafff, output); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *internal) placeBarcode(img barcode.Barcode, imageConf ImageConfig, darkness uint16, output io.Writer) error {
	imageSize := imageConf.Size
	if len(imageSize) > 1 {
		w, h := int(float64(imageSize[0])*t.scaleFactor), int(float64(imageSize[1])*t.scaleFactor)
		if scaled, err := barcode.Scale(img, w, h); err != nil {
			return err
		} else {
			img = scaled
		}
	}
	t.insertImage(img, imageConf, darkness, output)
	return nil
}

func (t *internal) placeImage(img image.Image, imageConf ImageConfig, darkness uint16, output io.Writer) error {
	imageSize := imageConf.Size
	if len(imageSize) > 1 {
		w, h := int(float64(imageSize[0])*t.scaleFactor), int(float64(imageSize[1])*t.scaleFactor)
		img = images.Resize(img, w, h)
	}
	t.insertImage(img, imageConf, darkness, output)
	return nil
}

func (t *internal) insertImage(img image.Image, imageConf ImageConfig, darkness uint16, output io.Writer) {
	x, y := scale(t.scaleFactor, imageConf.Position)

	imgWidth := float64(img.Bounds().Size().X)
	imgHeight := float64(img.Bounds().Size().Y)

	if imageConf.Center {
		x = int(float64(t.width)/2) - int(imgWidth/2)
		y = int(float64(t.height)/2) - int(imgHeight/2)
	}
	if imageConf.Top {
		y = 0
	} else if imageConf.Bottom {
		y = t.height - int(imgHeight)

	}
	if imageConf.Left {
		x = 0
	} else if imageConf.Right {
		x = t.width - int(imgWidth)
	}
	posx, posy := scale(t.scaleFactor, imageConf.Position)
	zpl.MoveCursor(x+posx+t.xOffset, y+posy+t.yOffset, output)

	zpl.PutImage(img, darkness, output)

}
