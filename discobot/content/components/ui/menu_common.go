package ui

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

func menuItemAction(item MenuItem) string {
	if item.Action != "" {
		return item.Action
	}
	return menuCommandAction(item.Command)
}

func menuCommandAction(command MenuCommand) string {
	if command.URL == "" {
		return ""
	}

	method := command.Method
	if method == "" {
		method = "POST"
	}
	return fmt.Sprintf("@discobotCommand(%s, {method: %s})", strconv.Quote(command.URL), strconv.Quote(method))
}

func menuPanelID(menuID string, path string) string {
	if path == "" {
		return menuID + "-panel"
	}
	return menuID + "-submenu-" + path
}

func menuItemPath(path string, index int) string {
	segment := strconv.Itoa(index)
	if path == "" {
		return segment
	}
	return path + "-" + segment
}

func menuPanelParentID(menuID string, parentPath string, submenu bool) string {
	if !submenu {
		return ""
	}
	return menuPanelID(menuID, parentPath)
}
