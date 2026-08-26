export class ApiError extends Error {
	status: number;

	constructor(status: number, message: string) {
		super(message);
		this.name = "ApiError";
		this.status = status;
	}
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
	const isFormData = init?.body instanceof FormData;
	const res = await fetch(path, {
		...init,
		credentials: "include",
		headers: {
			...(init?.body && !isFormData
				? { "Content-Type": "application/json" }
				: {}),
			...init?.headers,
		},
	});

	const text = await res.text();
	let body: unknown = null;
	if (text) {
		try {
			body = JSON.parse(text);
		} catch {
			body = null;
		}
	}

	if (!res.ok) {
		const message =
			body &&
			typeof body === "object" &&
			"error" in body &&
			typeof (body as { error: unknown }).error === "string"
				? (body as { error: string }).error
				: `request failed with status ${res.status}`;
		throw new ApiError(res.status, message);
	}

	return body as T;
}

export const apiClient = {
	get: <T>(path: string) => request<T>(path),
	post: <T>(path: string, data?: unknown) =>
		request<T>(path, {
			method: "POST",
			body: data !== undefined ? JSON.stringify(data) : undefined,
		}),
	put: <T>(path: string, data: unknown) =>
		request<T>(path, { method: "PUT", body: JSON.stringify(data) }),
	postForm: <T>(path: string, form: FormData) =>
		request<T>(path, { method: "POST", body: form }),
	delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
};
