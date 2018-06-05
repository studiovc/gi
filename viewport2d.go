// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gi

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"

	"github.com/goki/gi/oswin"
	"github.com/goki/ki"
	"github.com/goki/ki/bitflag"
	"github.com/goki/ki/kit"
	"github.com/goki/prof"
)

// A Viewport ALWAYS presents its children with a 0,0 - (Size.X, Size.Y)
// rendering area even if it is itself a child of another Viewport.  This is
// necessary for rendering onto the image that it provides.  This creates
// challenges for managing the different geometries in a coherent way, e.g.,
// events come through the Window in terms of the root VP coords.  Thus, nodes
// require a  WinBBox for events and a VpBBox for their parent Viewport.

// Viewport2D provides an image and a stack of Paint contexts for drawing onto the image
// with a convenience forwarding of the Paint methods operating on the current Paint
type Viewport2D struct {
	Node2DBase
	CSS     ki.Props    `desc:"cascading style sheet at this level -- these styles apply here and to everything below, until superceded -- use .class and #name Props elements to apply entire styles to given elements"`
	CSSAgg  ki.Props    `json:"-" xml:"-" desc:"aggregated css properties from all higher nodes down to me"`
	Fill    bool        `desc:"fill the viewport with background-color from style"`
	ViewBox ViewBox2D   `xml:"viewBox" desc:"viewbox within any parent Viewport2D"`
	Render  RenderState `json:"-" xml:"-" view:"-" desc:"render state for rendering"`
	Pixels  *image.RGBA `json:"-" xml:"-" view:"-" desc:"live pixels that we render into, from OSImage"`
	OSImage oswin.Image `json:"-" xml:"-" view:"-" desc:"the oswin.Image that owns our pixels"`
	Win     *Window     `json:"-" xml:"-" desc:"our parent window that we render into"`
}

var KiT_Viewport2D = kit.Types.AddType(&Viewport2D{}, Viewport2DProps)

var Viewport2DProps = ki.Props{
	"background-color": &Prefs.BackgroundColor,
}

// NewViewport2D creates a new OSImage with the specified width and height,
// and intializes the renderer etc
func NewViewport2D(width, height int) *Viewport2D {
	sz := image.Point{width, height}
	vp := &Viewport2D{
		ViewBox: ViewBox2D{Size: sz},
	}
	var err error
	vp.OSImage, err = oswin.TheApp.NewImage(sz)
	if err != nil {
		log.Printf("%v", err)
		return nil
	}
	vp.Pixels = vp.OSImage.RGBA()
	vp.Render.Init(width, height, vp.Pixels)
	return vp
}

// Resize resizes the viewport, creating a new image -- updates ViewBox Size
func (vp *Viewport2D) Resize(nwsz image.Point) {
	if nwsz.X == 0 || nwsz.Y == 0 {
		return
	}
	if vp.Pixels != nil {
		ib := vp.Pixels.Bounds().Size()
		if ib == nwsz {
			vp.ViewBox.Size = nwsz // make sure
			return                 // already good
		}
	}
	if vp.OSImage != nil {
		vp.OSImage.Release()
	}
	var err error
	vp.OSImage, err = oswin.TheApp.NewImage(nwsz)
	if err != nil {
		log.Printf("%v", err)
		return
	}
	vp.Pixels = vp.OSImage.RGBA()
	vp.Render.Init(nwsz.X, nwsz.Y, vp.Pixels)
	vp.ViewBox.Size = nwsz // make sure
	// fmt.Printf("vp %v resized to: %v, bounds: %v\n", vp.PathUnique(), nwsz, vp.OSImage.Bounds())
}

// these flags extend NodeBase NodeFlags to hold viewport state
const (
	// VpFlagPopup means viewport is a popup (menu or dialog) -- does not obey
	// parent bounds (otherwise does)
	VpFlagPopup NodeFlags = NodeFlagsN + iota

	// VpFlagMenu means viewport is serving as a popup menu -- affects how window
	// processes clicks
	VpFlagMenu

	// VpFlagPopupDestroyAll means that if this is a popup, then destroy all
	// the children when it is deleted -- otherwise children below the main
	// layout under the vp will not be destroyed -- it is up to the caller to
	// manage those (typically these are reusable assets)
	VpFlagPopupDestroyAll

	// VpFlagSVG mans that this viewport is an SVG viewport -- SVG elements
	// look for this for re-rendering
	VpFlagSVG
)

func (vp *Viewport2D) IsPopup() bool {
	return bitflag.Has(vp.Flag, int(VpFlagPopup))
}

func (vp *Viewport2D) IsMenu() bool {
	return bitflag.Has(vp.Flag, int(VpFlagMenu))
}

func (vp *Viewport2D) IsSVG() bool {
	return bitflag.Has(vp.Flag, int(VpFlagSVG))
}

// set our window pointer to point to the current window we are under
func (vp *Viewport2D) SetCurWin() {
	pwin := vp.ParentWindow()
	if pwin != nil { // ony update if non-nil -- otherwise we could be setting
		// temporarily to give access to DPI etc
		vp.Win = pwin
	}
}

////////////////////////////////////////////////////////////////////////////////////////
//  Main Rendering code

// UploadMainToWin is the update call for the main viewport for a window --
// calls UploadAllViewports in parent window, which uploads the main viewport
// and any active popups etc over the top of that
func (vp *Viewport2D) UploadMainToWin() {
	if vp.Win == nil {
		return
	}
	vp.Win.UploadAllViewports()
}

// UploadToWin uploads our viewport image into the parent window -- e.g., called
// by popups when updating separately
func (vp *Viewport2D) UploadToWin() {
	if vp.Win == nil {
		return
	}
	updt := vp.Win.UpdateStart()
	vp.Win.UploadVp(vp, vp.WinBBox.Min)
	vp.Win.UpdateEnd(updt)
}

// DrawIntoParent draws our viewport image into parent's image -- this is the
// typical way that a sub-viewport renders (e.g., svg boxes, icons, etc -- not popups)
func (vp *Viewport2D) DrawIntoParent(parVp *Viewport2D) {
	if vp.IsOverlay() { // don't check for any parent bounds etc -- just draw entire pixels
		if parVp == nil {
			return
		}
		r := vp.Pixels.Bounds()
		pos := vp.LayData.AllocPos.ToPoint() // get updated pos
		r = r.Add(pos)
		draw.Draw(parVp.Pixels, r, vp.Pixels, image.ZP, draw.Over)
		return
	}
	r := vp.ViewBox.Bounds()
	sp := image.ZP
	if vp.Par != nil { // use parents children bbox to determine where we can draw
		pgi, _ := KiToNode2D(vp.Par)
		nr := r.Intersect(pgi.ChildrenBBox2D())
		sp = nr.Min.Sub(r.Min)
		r = nr
	}
	draw.Draw(parVp.Pixels, r, vp.Pixels, sp, draw.Over)
}

// ReRender2DNode re-renders a specific node that has said it can re-render
func (vp *Viewport2D) ReRender2DNode(gni Node2D) {
	gn := gni.AsNode2D()
	pr := prof.Start("vp.ReRender2DNode")
	gn.Render2DTree()
	pr.End()
	if vp.Win != nil {
		updt := vp.Win.UpdateStart()
		vp.Win.UploadVpRegion(vp, gn.VpBBox, gn.WinBBox)
		vp.Win.UpdateEnd(updt)
	}
}

// ReRender2DAnchor re-renders an anchor node -- the KEY diff from
// ReRender2DNOde is that it calls ReRender2DTree and not just Render2DTree!
func (vp *Viewport2D) ReRender2DAnchor(gni Node2D) {
	gn := gni.AsNode2D()
	pr := prof.Start("vp.ReRender2DNode")
	gn.ReRender2DTree()
	pr.End()
	if vp.Win != nil {
		updt := vp.Win.UpdateStart()
		vp.Win.UploadVpRegion(vp, gn.VpBBox, gn.WinBBox)
		vp.Win.UpdateEnd(updt)
	}
}

// Delete this popup viewport -- has already been disconnected from window
// events and parent is nil -- called by window when a popup is deleted -- it
// destroys the vp and its main layout, see VpFlagPopupDestroyAll for whether
// children are destroyed
func (vp *Viewport2D) DeletePopup() {
	vp.Par = nil // disconnect from window -- it never actually owned us as a child
	if !bitflag.Has(vp.Flag, int(VpFlagPopupDestroyAll)) {
		// delete children of main layout prior to deleting the popup (e.g., menu items) so they don't get destroyed
		if len(vp.Kids) == 1 {
			cli, _ := KiToNode2D(vp.Child(0))
			ly := cli.AsLayout2D()
			if ly != nil {
				ly.DeleteChildren(false) // do NOT destroy children -- just delete them
			}
		}
	}
	vp.Destroy() // nuke everything else in us
}

////////////////////////////////////////////////////////////////////////////////////////
// Node2D interface

func (vp *Viewport2D) AsViewport2D() *Viewport2D {
	return vp
}

func (vp *Viewport2D) Init2D() {
	vp.SetCurWin()
	vp.Init2DBase()
	// we update oursleves whenever any node update event happens
	vp.NodeSig.Connect(vp.This, func(recvp, sendvp ki.Ki, sig int64, data interface{}) {
		rvpi, _ := KiToNode2D(recvp)
		rvp := rvpi.AsViewport2D()
		if Update2DTrace {
			fmt.Printf("Update: Viewport2D: %v full render due to signal: %v from node: %v\n", rvp.PathUnique(), ki.NodeSignals(sig), sendvp.PathUnique())
		}
		if !vp.IsDeleted() && !vp.IsDestroyed() {
			rvp.FullRender2DTree()
		}
	})
}

func (vp *Viewport2D) Style2D() {
	vp.SetCurWin()
	vp.Style2DWidget()
}

func (g *Viewport2D) CSSProps() (css, agg *ki.Props) {
	css = &g.CSS
	agg = &g.CSSAgg
	return
}

func (g *Viewport2D) StyleCSS(node Node2D) {
	StyleCSSWidget(node, g.CSS)
}

func (vp *Viewport2D) Size2D() {
	vp.InitLayout2D()
	// we listen to x,y styling for positioning within parent vp, if non-zero -- todo: only popup?
	pos := vp.Style.Layout.PosDots().ToPoint()
	if pos != image.ZP {
		vp.ViewBox.Min = pos
	}
	if !vp.IsSVG() && vp.ViewBox.Size != image.ZP {
		vp.LayData.AllocSize.SetPoint(vp.ViewBox.Size)
	}
}

func (vp *Viewport2D) Layout2D(parBBox image.Rectangle) {
	vp.Layout2DBase(parBBox, true)
	vp.Layout2DChildren()
}

func (vp *Viewport2D) BBox2D() image.Rectangle {
	// viewport ignores any parent parent bbox info!
	if vp.Pixels == nil || !vp.IsPopup() { // non-popups use allocated sizes via layout etc
		if !vp.LayData.AllocSize.IsZero() {
			asz := vp.LayData.AllocSize.ToPointCeil()
			vp.Resize(asz)
		} else if vp.Pixels == nil {
			vp.Resize(image.Point{64, 64}) // gotta have something..
		}
	}
	return vp.Pixels.Bounds()
}

func (vp *Viewport2D) ComputeBBox2D(parBBox image.Rectangle, delta image.Point) {
	vp.VpBBox = vp.Pixels.Bounds()
	vp.SetWinBBox()    // this adds all PARENT offsets
	if !vp.IsPopup() { // non-popups use allocated positions
		vp.ViewBox.Min = vp.LayData.AllocPos.ToPointFloor()
	}
	vp.WinBBox = vp.WinBBox.Add(vp.ViewBox.Min)
	// fmt.Printf("Viewport: %v bbox: %v vpBBox: %v winBBox: %v\n", vp.PathUnique(), vp.BBox, vp.VpBBox, vp.WinBBox)
}

func (vp *Viewport2D) ChildrenBBox2D() image.Rectangle {
	return vp.VpBBox
}

// RenderViewport2D is the render action for the viewport itself -- either
// uploads image to window or draws into parent viewport
func (vp *Viewport2D) RenderViewport2D() {
	if vp.IsPopup() { // popup has a parent that is the window
		vp.SetCurWin()
		if Render2DTrace {
			fmt.Printf("Render: %v at %v Popup UploadToWin\n", vp.PathUnique())
		}
		vp.UploadToWin()
	} else if vp.Viewport != nil { // sub-vp
		if Render2DTrace {
			fmt.Printf("Render: %v at %v DrawIntoParent\n", vp.PathUnique(), vp.VpBBox)
		}
		vp.DrawIntoParent(vp.Viewport)
	} else { // we are the main vp
		if Render2DTrace {
			fmt.Printf("Render: %v at %v UploadMainToWin\n", vp.PathUnique(), vp.VpBBox)
		}
		vp.UploadMainToWin()
	}
}

// we use our own render for these -- Viewport member is our parent!
func (vp *Viewport2D) PushBounds() bool {
	if vp.IsOverlay() {
		if vp.Viewport != nil {
			vp.Viewport.Render.PushBounds(vp.Viewport.Pixels.Bounds())
		}
		return true
	}
	if vp.VpBBox.Empty() {
		return false
	}
	// if we are completely invisible, no point in rendering..
	if vp.Viewport != nil {
		wbi := vp.WinBBox.Intersect(vp.Viewport.WinBBox)
		if wbi.Empty() {
			// fmt.Printf("not rendering vp %v bc empty winbox -- ours: %v par: %v\n", vp.Nm, vp.WinBBox, vp.Viewport.WinBBox)
			return false
		}
	}
	rs := &vp.Render
	rs.PushBounds(vp.VpBBox)
	if Render2DTrace {
		fmt.Printf("Render: %v at %v\n", vp.PathUnique(), vp.VpBBox)
	}
	return true
}

func (vp *Viewport2D) PopBounds() {
	rs := &vp.Render
	rs.PopBounds()
}

func (vp *Viewport2D) Move2D(delta image.Point, parBBox image.Rectangle) {
	vp.Move2DBase(delta, parBBox)
	vp.Move2DChildren(image.ZP) // reset delta here -- we absorb the delta in our placement relative to the parent
}

func (vp *Viewport2D) FillViewport() {
	vp.Paint.FillBox(&vp.Render, Vec2DZero, NewVec2DFmPoint(vp.ViewBox.Size), &vp.Style.Background.Color)
}

func (vp *Viewport2D) Render2D() {
	if vp.PushBounds() {
		if vp.Fill {
			vp.FillViewport()
		}
		vp.Render2DChildren() // we must do children first, then us!
		vp.RenderViewport2D() // update our parent image
		vp.PopBounds()
	}
}

func (vp *Viewport2D) ReRender2D() (node Node2D, layout bool) {
	node = vp.This.(Node2D)
	layout = false
	return
}

func (g *Viewport2D) FocusChanged2D(gotFocus bool) {
}

////////////////////////////////////////////////////////////////////////////////////////
//  Signal Handling

// each node calls this signal method to notify its parent viewport whenever it changes, causing a re-render
func SignalViewport2D(vpki, send ki.Ki, sig int64, data interface{}) {
	vpgi, ok := vpki.(Node2D)
	if !ok {
		return
	}
	vp := vpgi.AsViewport2D()
	if vp == nil { // should not happen -- should only be called on viewports
		return
	}
	gii, gi := KiToNode2D(send)
	if gii == nil { // should not happen
		return
	}
	if gi.IsDeleted() || gi.IsDestroyed() { // skip these for sure
		return
	}
	if gi.IsUpdatingAtomic() {
		log.Printf("ERROR SignalViewport2D updating node %v with Updating flag set\n", gi.PathUnique())
		return
	}

	if Update2DTrace {
		fmt.Printf("Update: Viewport2D: %v rendering due to signal: %v from node: %v\n", vp.PathUnique(), ki.NodeSignals(sig), send.PathUnique())
	}

	fullRend := false
	if sig == int64(ki.NodeSignalUpdated) {
		dflags := data.(int64)
		vlupdt := bitflag.HasMask(dflags, ki.ValUpdateFlagsMask)
		strupdt := bitflag.HasMask(dflags, ki.StruUpdateFlagsMask)
		if vlupdt && !strupdt {
			fullRend = false
		} else if strupdt {
			fullRend = true
		}
	} else {
		fullRend = true
	}

	if fullRend {
		if Update2DTrace {
			fmt.Printf("Update: Viewport2D: %v FullRender2DTree (structural changes)\n", vp.PathUnique())
		}
		vp.FullRender2DTree()
	} else {
		rr, layout := gii.ReRender2D()
		if rr != nil {
			rrn := rr.AsNode2D()
			if Update2DTrace {
				fmt.Printf("Update: Viewport2D: %v ReRender2D on %v, layout: %v\n", vp.PathUnique(), rrn.PathUnique(), layout)
			}
			if layout {
				rrn.Layout2DTree()
			}
			vp.ReRender2DNode(rr)
		} else {
			anchor := gi.ParentReRenderAnchor()
			if anchor != nil {
				if Update2DTrace {
					fmt.Printf("Update: Viewport2D: %v ReRender2D nil, found anchor, styling: %v, then doing ReRender2DTree on: %v\n", vp.PathUnique(), gi.PathUnique(), anchor.PathUnique())
				}
				gi.Style2DTree() // restyle only from affected node downward
				vp.ReRender2DAnchor(anchor.AsNode2D())
			} else {
				if Update2DTrace {
					fmt.Printf("Update: Viewport2D: %v ReRender2D nil, styling: %v, then doing ReRender2DTree on us\n", vp.PathUnique(), gi.PathUnique())
				}
				gi.Style2DTree()    // restyle only from affected node downward
				vp.ReRender2DTree() // need to re-render entirely from us
			}
		}
	}
	// don't do anything on deleting or destroying, and
}

////////////////////////////////////////////////////////////////////////////////////////
//  Overlay rendering

// RenderOverlays is main call from window for OverlayVp to render overlay nodes within it
func (vp *Viewport2D) RenderOverlays(wsz image.Point) {
	vp.SetAsOverlay()
	vp.Resize(wsz)

	// fill to transparent
	draw.Draw(vp.Pixels, vp.Pixels.Bounds(), &image.Uniform{color.Transparent}, image.ZP, draw.Src)

	// just do top-level objects here
	for _, k := range vp.Kids {
		gii, gi := KiToNode2D(k)
		if gii == nil {
			continue
		}
		if !gi.IsOverlay() { // has not been initialized
			gi.SetAsOverlay()
			gii.Init2D()
			gii.Style2D()
		}
		// note: skipping sizing, layout -- use simple elements here, esp Bitmap
		gii.Render2D()
	}
}

//////////////////////////////////////////////////////////////////////////////////
//  Image utilities

// SavePNG encodes the image as a PNG and writes it to disk.
func (vp *Viewport2D) SavePNG(path string) error {
	return SavePNG(path, vp.Pixels)
}

// EncodePNG encodes the image as a PNG and writes it to the provided io.Writer.
func (vp *Viewport2D) EncodePNG(w io.Writer) error {
	return png.Encode(w, vp.Pixels)
}
