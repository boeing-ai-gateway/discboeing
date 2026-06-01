package helpers

import (
	"fmt"
	"strconv"

	"github.com/a-h/templ"
)

type MenuCheckState int

const (
	MenuCheckNone MenuCheckState = iota
	MenuCheckUnchecked
	MenuCheckChecked
)

type MenuData struct {
	ID            string
	Label         string
	Trigger       templ.Component
	Items         []MenuItem
	AnchorToClick bool
}

type MenuItem struct {
	Label           string
	CheckState      MenuCheckState
	SeparatorBefore bool
	Command         MenuCommand
	Action          string
	Children        []MenuItem
}

type MenuCommand struct {
	Method string
	URL    string
}

func MenuItemAction(item MenuItem) string {
	if item.Action != "" {
		return item.Action
	}
	return MenuCommandAction(item.Command)
}

func MenuCommandAction(command MenuCommand) string {
	if command.URL == "" {
		return ""
	}

	method := command.Method
	if method == "" {
		method = "POST"
	}
	return fmt.Sprintf("@discobotCommand(%s, {method: %s})", strconv.Quote(command.URL), strconv.Quote(method))
}

func MenuPanelID(menuID string, path string) string {
	if path == "" {
		return menuID + "-panel"
	}
	return menuID + "-submenu-" + path
}

func MenuItemPath(path string, index int) string {
	segment := strconv.Itoa(index)
	if path == "" {
		return segment
	}
	return path + "-" + segment
}

func MenuPanelParentID(menuID string, parentPath string, submenu bool) string {
	if !submenu {
		return ""
	}
	return MenuPanelID(menuID, parentPath)
}
