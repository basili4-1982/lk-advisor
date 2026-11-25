// Package explain builds the human-readable reason for a recommendation.
//
// The template works offline; the LLM only rephrases facts that are already
// in the evidence. Every number in the answer is checked against evidence.
package explain
