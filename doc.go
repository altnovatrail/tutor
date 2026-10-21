// Package tutor is the AI side of NovaTrail's tutor, a library that api calls in its own
// process. It reads the learner's work into an observation, reviews the working and words the
// tutor's replies, through the AI routes and models it admits, and reports each call's usage.
// It decides no verdict and stores nothing, and its requests carry no identifier of the learner
// or the account.
//
// The AI providers are reached only through the port in package provider. The eval runner,
// cmd/tutor-eval, runs on the same code.
package tutor
