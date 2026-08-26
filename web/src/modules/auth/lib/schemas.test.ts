import { describe, expect, it } from "vitest";
import { loginSchema, signupSchema } from "./schemas";

describe("signupSchema", () => {
	it("mirrors the backend username regex (^[a-z0-9_]{3,25}$)", () => {
		expect(
			signupSchema.safeParse({
				email: "a@b.com",
				username: "ab",
				password: "password1",
			}).success,
		).toBe(false);
		expect(
			signupSchema.safeParse({
				email: "a@b.com",
				username: "Has-Caps",
				password: "password1",
			}).success,
		).toBe(false);
		expect(
			signupSchema.safeParse({
				email: "a@b.com",
				username: "valid_user1",
				password: "password1",
			}).success,
		).toBe(true);
	});

	it("mirrors the backend minimum password length of 8", () => {
		expect(
			signupSchema.safeParse({
				email: "a@b.com",
				username: "valid_user",
				password: "short1",
			}).success,
		).toBe(false);
		expect(
			signupSchema.safeParse({
				email: "a@b.com",
				username: "valid_user",
				password: "longenough1",
			}).success,
		).toBe(true);
	});

	it("rejects an invalid email", () => {
		expect(
			signupSchema.safeParse({
				email: "not-an-email",
				username: "valid_user",
				password: "longenough1",
			}).success,
		).toBe(false);
	});
});

describe("loginSchema", () => {
	it("requires a non-empty password but does not enforce the signup minimum", () => {
		expect(
			loginSchema.safeParse({ email: "a@b.com", password: "" }).success,
		).toBe(false);
		expect(
			loginSchema.safeParse({ email: "a@b.com", password: "x" }).success,
		).toBe(true);
	});
});
