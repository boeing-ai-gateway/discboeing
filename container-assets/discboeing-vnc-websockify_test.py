#!/usr/bin/env python3

import importlib.machinery
import importlib.util
import pathlib
import subprocess
import unittest
from unittest import mock


MODULE_PATH = pathlib.Path(__file__).with_name("discboeing-vnc-websockify")


loader = importlib.machinery.SourceFileLoader("discboeing_vnc_websockify", str(MODULE_PATH))
spec = importlib.util.spec_from_loader(loader.name, loader)
wrapper = importlib.util.module_from_spec(spec)
loader.exec_module(wrapper)


class RequestedDesktopSizeTest(unittest.TestCase):
    def test_missing_size_returns_none(self):
        self.assertIsNone(wrapper.requested_desktop_size("/"))
        self.assertIsNone(wrapper.requested_desktop_size("/?token=abc"))

    def test_valid_size_is_returned(self):
        self.assertEqual(wrapper.requested_desktop_size("/?x=1280&y=768"), (1280, 768))

    def test_dimensions_are_rounded_up_to_resize_bucket(self):
        self.assertEqual(wrapper.requested_desktop_size("/?x=1001&y=701"), (1024, 704))

    def test_requires_both_dimensions(self):
        with self.assertRaisesRegex(wrapper.DesktopResizeError, "both x and y"):
            wrapper.requested_desktop_size("/?x=1280")
        with self.assertRaisesRegex(wrapper.DesktopResizeError, "both x and y"):
            wrapper.requested_desktop_size("/?y=720")

    def test_rejects_repeated_dimensions(self):
        with self.assertRaisesRegex(wrapper.DesktopResizeError, "specified once"):
            wrapper.requested_desktop_size("/?x=1280&x=1288&y=720")

    def test_rejects_non_integer_dimensions(self):
        with self.assertRaisesRegex(wrapper.DesktopResizeError, "positive integer"):
            wrapper.requested_desktop_size("/?x=abcd&y=720")
        with self.assertRaisesRegex(wrapper.DesktopResizeError, "positive integer"):
            wrapper.requested_desktop_size("/?x=-128&y=720")

    def test_rejects_out_of_bounds_dimensions(self):
        with self.assertRaisesRegex(wrapper.DesktopResizeError, "x must be between"):
            wrapper.requested_desktop_size("/?x=639&y=720")
        with self.assertRaisesRegex(wrapper.DesktopResizeError, "y must be between"):
            wrapper.requested_desktop_size("/?x=1280&y=2001")

    def test_rejects_oversized_path_query_and_dimensions(self):
        with self.assertRaisesRegex(wrapper.DesktopResizeError, "path is too long"):
            wrapper.requested_desktop_size("/" + "a" * wrapper.MAX_PATH_LENGTH + "?x=1280&y=720")
        with self.assertRaisesRegex(wrapper.DesktopResizeError, "query is too long"):
            wrapper.requested_desktop_size("/?" + "a" * (wrapper.MAX_QUERY_LENGTH + 1))
        with self.assertRaisesRegex(wrapper.DesktopResizeError, "x is too long"):
            wrapper.requested_desktop_size("/?x=01280&y=720")

    def test_rejects_too_many_query_fields(self):
        query = "&".join(f"p{i}=1" for i in range(wrapper.MAX_QUERY_FIELDS + 1))
        with self.assertRaisesRegex(wrapper.DesktopResizeError, "too many fields"):
            wrapper.requested_desktop_size("/?" + query)


class ResizeDesktopTest(unittest.TestCase):
    def setUp(self):
        wrapper._mode_cache.clear()

    def test_skips_resize_when_already_at_requested_size(self):
        with mock.patch.object(wrapper, "connected_output") as connected_output, mock.patch.object(
            wrapper, "current_size", return_value=(1280, 720)
        ) as current_size, mock.patch.object(wrapper, "ensure_mode") as ensure_mode, mock.patch.object(
            wrapper, "run"
        ) as run:
            wrapper.resize_desktop(1280, 720)

        connected_output.assert_called_once_with()
        current_size.assert_called_once_with()
        ensure_mode.assert_not_called()
        run.assert_not_called()

    def test_creates_mode_and_applies_it_before_proxying(self):
        with mock.patch.object(
            wrapper, "connected_output", return_value="DUMMY0"
        ) as connected_output, mock.patch.object(
            wrapper, "current_size", return_value=(1280, 720)
        ) as current_size, mock.patch.object(wrapper, "ensure_mode") as ensure_mode, mock.patch.object(
            wrapper, "run"
        ) as run:
            wrapper.resize_desktop(1024, 704)

        connected_output.assert_called_once_with()
        current_size.assert_called_once_with()
        ensure_mode.assert_called_once_with("DUMMY0", "1024x704_60.00", 1024, 704)
        run.assert_called_once_with([wrapper.XRANDR, "--output", "DUMMY0", "--mode", "1024x704_60.00"])

    def test_rejects_new_modes_after_cache_limit(self):
        for index in range(wrapper.MAX_DYNAMIC_MODES):
            wrapper._mode_cache.add(("DUMMY0", f"cached-{index}"))
        result = subprocess.CompletedProcess([], 0, stdout="DUMMY0 connected\n", stderr="")
        with mock.patch.object(wrapper, "run", return_value=result), mock.patch.object(
            wrapper, "generate_modeline"
        ) as generate_modeline, mock.patch.object(wrapper, "add_new_mode") as add_new_mode:
            with self.assertRaisesRegex(wrapper.DesktopResizeError, "mode limit"):
                wrapper.ensure_mode("DUMMY0", "2048x1152_60.00", 2048, 1152)

        generate_modeline.assert_not_called()
        add_new_mode.assert_not_called()

    def test_existing_modes_can_be_added_after_cache_limit(self):
        for index in range(wrapper.MAX_DYNAMIC_MODES):
            wrapper._mode_cache.add(("DUMMY0", f"cached-{index}"))
        result = subprocess.CompletedProcess([], 0, stdout="2048x1152_60.00\n", stderr="")
        add_result = subprocess.CompletedProcess([], 0, stdout="", stderr="")
        with mock.patch.object(wrapper, "run", side_effect=[result, add_result]) as run:
            wrapper.ensure_mode("DUMMY0", "2048x1152_60.00", 2048, 1152)

        self.assertNotIn(("DUMMY0", "2048x1152_60.00"), wrapper._mode_cache)
        self.assertEqual(run.call_args_list[0], mock.call([wrapper.XRANDR, "--query"]))
        self.assertEqual(
            run.call_args_list[1],
            mock.call([wrapper.XRANDR, "--addmode", "DUMMY0", "2048x1152_60.00"], check=False),
        )

    def test_rejects_concurrent_resize(self):
        self.assertTrue(wrapper._resize_lock.acquire(blocking=False))
        try:
            with self.assertRaisesRegex(wrapper.DesktopResizeError, "already in progress"):
                wrapper.resize_desktop(1280, 720)
        finally:
            wrapper._resize_lock.release()

    def test_ensure_mode_uses_cache(self):
        wrapper._mode_cache.add(("DUMMY0", "1280x720_60.00"))
        with mock.patch.object(wrapper, "run") as run:
            wrapper.ensure_mode("DUMMY0", "1280x720_60.00", 1280, 720)
        run.assert_not_called()

    def test_add_new_mode_ignores_already_exists(self):
        result = subprocess.CompletedProcess([], 1, stdout="", stderr="mode already exists")
        with mock.patch.object(wrapper, "run", return_value=result) as run:
            wrapper.add_new_mode("1280x720_60.00", ["1", "2"])
        run.assert_called_once_with([wrapper.XRANDR, "--newmode", "1280x720_60.00", "1", "2"], check=False)

    def test_current_size_rejects_empty_xrandr_output(self):
        result = subprocess.CompletedProcess([], 0, stdout="", stderr="")
        with mock.patch.object(wrapper, "run", return_value=result):
            with self.assertRaisesRegex(wrapper.DesktopResizeError, "could not read current display size"):
                wrapper.current_size()


class RunCommandTest(unittest.TestCase):
    def test_uses_timeout_absolute_command_and_minimal_env(self):
        result = subprocess.CompletedProcess([wrapper.XRANDR], 0, stdout="", stderr="")
        with mock.patch.object(subprocess, "run", return_value=result) as run:
            self.assertIs(wrapper.run([wrapper.XRANDR, "--query"]), result)

        run.assert_called_once_with(
            [wrapper.XRANDR, "--query"],
            check=False,
            env={"DISPLAY": wrapper.DISPLAY, "PATH": "/usr/bin:/bin"},
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=wrapper.COMMAND_TIMEOUT_SECONDS,
        )

    def test_timeout_is_reported_as_resize_error(self):
        with mock.patch.object(subprocess, "run", side_effect=subprocess.TimeoutExpired(wrapper.XRANDR, 3)):
            with self.assertRaisesRegex(wrapper.DesktopResizeError, "xrandr timed out"):
                wrapper.run([wrapper.XRANDR, "--query"])


class HandlerCloseTest(unittest.TestCase):
    def test_resize_validation_errors_use_generic_close_reason(self):
        class FakeClose(Exception):
            pass

        handler = object.__new__(wrapper.DesktopProxyRequestHandler)
        handler.path = "/?x=secret&y=720"
        handler.CClose = FakeClose
        handler.log_message = mock.Mock()

        with self.assertRaises(FakeClose) as caught:
            handler.new_websocket_client()

        self.assertEqual(caught.exception.args, (1008, wrapper.INVALID_RESIZE_REASON))
        handler.log_message.assert_called_once()

    def test_unexpected_resize_errors_use_generic_close_reason(self):
        class FakeClose(Exception):
            pass

        handler = object.__new__(wrapper.DesktopProxyRequestHandler)
        handler.path = "/?x=1280&y=720"
        handler.CClose = FakeClose
        handler.log_message = mock.Mock()

        with mock.patch.object(wrapper, "resize_desktop", side_effect=RuntimeError("secret detail")):
            with self.assertRaises(FakeClose) as caught:
                handler.new_websocket_client()

        self.assertEqual(caught.exception.args, (1011, wrapper.RESIZE_FAILED_REASON))
        handler.log_message.assert_called_once()


if __name__ == "__main__":
    unittest.main()
