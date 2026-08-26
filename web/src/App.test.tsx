import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import App from "./App";

describe("App", () => {
	it("renders the home page by default", async () => {
		render(<App />);
		expect((await screen.findAllByText("Ao Vivo")).length).toBeGreaterThan(0);
	});
});
