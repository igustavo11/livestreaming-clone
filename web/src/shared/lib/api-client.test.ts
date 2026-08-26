import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, apiClient } from "./api-client";

function mockFetchOnce(status: number, body: unknown) {
	vi.stubGlobal(
		"fetch",
		vi.fn().mockResolvedValue(
			new Response(JSON.stringify(body), {
				status,
				headers: { "Content-Type": "application/json" },
			}),
		),
	);
}

afterEach(() => {
	vi.unstubAllGlobals();
});

describe("apiClient", () => {
	it("returns the parsed body on a 2xx response", async () => {
		mockFetchOnce(200, { ok: true });
		await expect(apiClient.get("/api/whatever")).resolves.toEqual({ ok: true });
	});

	it("throws ApiError with the backend's {error} message on failure", async () => {
		mockFetchOnce(401, { error: "authentication required" });
		await expect(apiClient.get("/api/me/channel")).rejects.toMatchObject({
			name: "ApiError",
			status: 401,
			message: "authentication required",
		});
	});

	it("falls back to a generic message when the error body has no {error} field", async () => {
		mockFetchOnce(500, {});
		try {
			await apiClient.get("/api/whatever");
			throw new Error("expected apiClient.get to reject");
		} catch (err) {
			expect(err).toBeInstanceOf(ApiError);
			expect((err as ApiError).message).toBe("request failed with status 500");
		}
	});

	it("sends credentials and a JSON content-type for a POST body", async () => {
		mockFetchOnce(200, { status: "ok" });
		await apiClient.post("/api/auth/login", {
			email: "a@b.com",
			password: "x",
		});

		const [, init] = vi.mocked(fetch).mock.calls[0];
		if (!init) throw new Error("expected fetch to receive an init object");
		expect(init.credentials).toBe("include");
		expect((init.headers as Record<string, string>)["Content-Type"]).toBe(
			"application/json",
		);
		expect(init.body).toBe(JSON.stringify({ email: "a@b.com", password: "x" }));
	});
});
