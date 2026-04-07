#!/usr/bin/python3
import subprocess

import gi
gi.require_version("Gtk", "3.0")
gi.require_version("AyatanaAppIndicator3", "0.1")
from gi.repository import AyatanaAppIndicator3 as AppIndicator3
from gi.repository import GLib, Gtk

COMMAND = ["/usr/local/bin/screentimectl", "status", "--compact"]


def status_text():
    try:
        return subprocess.check_output(COMMAND, text=True, timeout=2).strip()
    except Exception:
        return "Screen time unavailable"


indicator = AppIndicator3.Indicator.new(
    "screentimectl",
    "appointment-soon-symbolic",
    AppIndicator3.IndicatorCategory.APPLICATION_STATUS,
)
indicator.set_status(AppIndicator3.IndicatorStatus.ACTIVE)

menu = Gtk.Menu()

remaining_item = Gtk.MenuItem(label="")
remaining_item.set_sensitive(False)
menu.append(remaining_item)

refresh_item = Gtk.MenuItem(label="Refresh")
menu.append(refresh_item)

quit_item = Gtk.MenuItem(label="Quit")
quit_item.connect("activate", Gtk.main_quit)
menu.append(quit_item)


def refresh(*_):
    text = status_text()
    remaining_item.set_label(text)
    indicator.set_label(text, text)
    return True


refresh_item.connect("activate", refresh)
menu.show_all()
indicator.set_menu(menu)

refresh()
GLib.timeout_add_seconds(60, refresh)
Gtk.main()
