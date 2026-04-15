import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { debounce } from "./debounce";

describe("debounce", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("does not call fn immediately", () => {
    const fn = vi.fn();
    const debounced = debounce(fn, 100);
    debounced();
    expect(fn).not.toHaveBeenCalled();
  });

  it("calls fn after the delay", () => {
    const fn = vi.fn();
    const debounced = debounce(fn, 100);
    debounced();
    vi.advanceTimersByTime(100);
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it("resets the timer on each call", () => {
    const fn = vi.fn();
    const debounced = debounce(fn, 100);

    debounced();
    vi.advanceTimersByTime(50);
    debounced();
    vi.advanceTimersByTime(50);
    // Only 50ms since the second call — fn should not have fired yet
    expect(fn).not.toHaveBeenCalled();

    vi.advanceTimersByTime(50);
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it("calls fn only once for rapid repeated calls", () => {
    const fn = vi.fn();
    const debounced = debounce(fn, 100);

    for (let i = 0; i < 10; i++) {
      debounced();
    }
    vi.advanceTimersByTime(100);
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it("passes the latest arguments to fn", () => {
    const fn = vi.fn();
    const debounced = debounce(fn, 100);

    debounced("first");
    debounced("second");
    debounced("third");
    vi.advanceTimersByTime(100);

    expect(fn).toHaveBeenCalledWith("third");
  });

  it("can be called again after the delay has elapsed", () => {
    const fn = vi.fn();
    const debounced = debounce(fn, 100);

    debounced();
    vi.advanceTimersByTime(100);
    expect(fn).toHaveBeenCalledTimes(1);

    debounced();
    vi.advanceTimersByTime(100);
    expect(fn).toHaveBeenCalledTimes(2);
  });

  it("works with zero delay", () => {
    const fn = vi.fn();
    const debounced = debounce(fn, 0);
    debounced();
    vi.advanceTimersByTime(0);
    expect(fn).toHaveBeenCalledTimes(1);
  });
});
