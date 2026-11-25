// Package recommend turns a client profile into ranked recommendations.
//
// Pipeline: intent -> candidates -> rules -> scoring -> diversity -> top-3.
// Business logic is added during the sprint; see algorithm.md.
package recommend
