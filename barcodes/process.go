package barcodes

import (
	"fmt"
	"image"
	"io"

	"github.com/codenaut/barcoder/images"
	"github.com/codenaut/barcoder/zpl"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/qr"
	"github.com/fogleman/gg"
	"gopkg.in/urfave/cli.v1"
)

const (
	typeQr = iota
	typeCode128
)

type barcodeType int

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

	Image       []ImageFileConfig
	Text        []TextConfig
	Qr          []BarcodeProperties
	Code128     []BarcodeProperties
	Code128Text []TextConfig
}
type ImageFileConfig struct {
	File       string
	Input      int
	Darkness   uint16
	Properties ImageConfig
}
type TextConfig struct {
	Input      int
	Value      string
	Font       string
	FontSize   float64
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
	Rotate   float64
}

func New(config BarcodeConfig, xOffset int, yOffset int) *internal {

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

func (t *internal) processText(value string, txt TextConfig, output io.Writer, args cli.Args) error {
	str := value
	if str == "" {
		str = txt.Value
	}
	if str == "" {
		str = args.Get(txt.Input)
	}
	ctx := gg.NewContext(t.width, t.height)
	font := txt.Font
	fontSize := txt.FontSize
	if font == "" {
		font = "/Library/Fonts/Verdana.ttf"
	}
	if fontSize == 0 {
		fontSize = 72
	}
	if err := ctx.LoadFontFace(font, fontSize); err != nil {
		return err
	}
	w, h := ctx.MeasureString(str)

	strCtx := gg.NewContext(int(w*1.1), int(h*1.4))
	if err := strCtx.LoadFontFace(font, fontSize); err != nil {
		return err
	}
	strCtx.SetColor(image.White)
	strCtx.Clear()
	strCtx.SetColor(image.Black)
	x, y := 0, 0
	strCtx.DrawString(str, float64(x), float64(y)+h)
	img := strCtx.Image()

	t.insertImage(img, txt.Properties, 0xafff, output)
	return nil
}

func (t *internal) Process(output io.Writer, args cli.Args) error {

	zpl.Start(output)
	for _, img := range t.config.Image {
		filename := img.File
		if filename == "" {
			filename = args.Get(img.Input)
		}
		if i, err := images.OpenPng(filename); err != nil {
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
	for _, txt := range t.config.Code128Text {
		v := txt.Value
		if v == "" {
			v = args.Get(txt.Input)
		}
		if barcode, err := code128.Encode(v); err != nil {
			return err
		} else {
			if err := t.processText(fmt.Sprintf("%s%d", v, barcode.CheckSum()), txt, output, args); err != nil {
				return err
			}
		}
	}
	for _, txt := range t.config.Text {
		if err := t.processText("", txt, output, args); err != nil {
			return err
		}
	}

	t.processBarcodes(t.config.Qr, typeQr, args, output)
	t.processBarcodes(t.config.Code128, typeCode128, args, output)

	zpl.End(output)
	return nil
}
func (t *internal) processBarcodes(confs []BarcodeProperties, bType barcodeType, args cli.Args, output io.Writer) error {
	for _, conf := range confs {
		str := conf.Value
		if str == "" {
			str = args.Get(conf.Input)
		}
		var bCode barcode.Barcode
		var err error
		switch bType {
		case typeQr:
			bCode, err = qr.Encode(str, qr.M, qr.Auto)
		case typeCode128:
			bCode, err = code128.Encode(str)
		default:
			return fmt.Errorf("Bad barcode type: %d", bType)
		}

		if err != nil {
			return err
		} else {
			if err := t.placeBarcode(bCode, conf.Properties, 0xafff, output); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *internal) placeBarcode(img barcode.Barcode, imageConf ImageConfig, darkness uint16, output io.Writer) error {
	imageSize := imageConf.Size
	if len(imageSize) > 1 {
		origW, origH := img.Bounds().Size().X, img.Bounds().Size().Y
		w, h := int(float64(imageSize[0])*t.scaleFactor), int(float64(imageSize[1])*t.scaleFactor)
		if w < origW {
			w = origW
		}
		if h < origH {
			h = origH
		}

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

	if imageConf.Rotate != 0.0 {
		img = images.Rotate(img, imageConf.Rotate)
	}
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
