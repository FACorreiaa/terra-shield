import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import { z } from "zod";

import { readStored, subscribeStored, writeStored } from "./storage.ts";

const KEY = "shield.test";
const schema = z.object({ excluded: z.array(z.string()) });
const fallback = { excluded: [] };

function reset(): void {
  vi.restoreAllMocks();
  localStorage.clear();
}

describe(readStored, () => {
  afterEach(reset);

  it("reads back what was written at the same version", () => {
    writeStored(KEY, 1, { excluded: ["wind"] });
    expect(localStorage.getItem(KEY)).toBe('{"v":1,"data":{"excluded":["wind"]}}');
    expect(readStored(KEY, 1, schema, fallback)).toStrictEqual({ excluded: ["wind"] });
  });

  it("gives the fallback for nothing stored", () => {
    expect(readStored(KEY, 1, schema, fallback)).toBe(fallback);
  });

  it("gives the fallback for another version", () => {
    writeStored(KEY, 1, { excluded: ["wind"] });
    expect(readStored(KEY, 2, schema, fallback)).toBe(fallback);
  });

  it.each([
    ["corrupt JSON", "{not json"],
    ["a bare value", '"wind"'],
    ["no version", '{"data":{"excluded":[]}}'],
    ["data the schema refuses", '{"v":1,"data":{"excluded":"wind"}}'],
  ])("gives the fallback for %s", (_name, raw) => {
    localStorage.setItem(KEY, raw);
    expect(readStored(KEY, 1, schema, fallback)).toBe(fallback);
  });

  it("gives the fallback when storage throws", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new DOMException("blocked", "SecurityError");
    });
    expect(readStored(KEY, 1, schema, fallback)).toBe(fallback);
  });

  it("gives the fallback when storage itself is out of reach", () => {
    vi.spyOn(globalThis, "localStorage", "get").mockImplementation(() => {
      throw new DOMException("blocked", "SecurityError");
    });
    expect(readStored(KEY, 1, schema, fallback)).toBe(fallback);
    expect(() => {
      writeStored(KEY, 1, fallback);
    }).not.toThrow();
  });
});

describe(writeStored, () => {
  afterEach(reset);

  it("does nothing when storage is full", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("full", "QuotaExceededError");
    });
    expect(() => {
      writeStored(KEY, 1, fallback);
    }).not.toThrow();
  });
});

describe(subscribeStored, () => {
  afterEach(reset);

  it("hears another tab change the key, or clear storage, until unsubscribed", () => {
    const onChange = vi.fn<() => void>();
    const unsubscribe = subscribeStored(KEY, onChange);
    globalThis.dispatchEvent(new StorageEvent("storage", { key: KEY }));
    globalThis.dispatchEvent(new StorageEvent("storage", { key: "other" }));
    globalThis.dispatchEvent(new StorageEvent("storage"));
    expect(onChange).toHaveBeenCalledTimes(2);
    unsubscribe();
    globalThis.dispatchEvent(new StorageEvent("storage", { key: KEY }));
    expect(onChange).toHaveBeenCalledTimes(2);
  });
});
