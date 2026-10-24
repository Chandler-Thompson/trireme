# Modules

The doctrine is split into modules so each part can change without touching the rest.
For every module, a project chooses one of three:

1. **Inherit** the module unchanged.
2. **Inherit with deviations.** The project lists each rule it changes, with a reason.
   Everything it does not list still binds.
3. **Replace** the module with a different one: an alternative shipped with Trireme, one
   imported from a file, or one written for the project.

The same mechanism serves A/B testing: two projects, or two periods of one project, can
run different versions of a module and be compared.

## Module header

Every module opens with the same header:

- **Module:** its id.
- **Version:** bumped on every change to the module.
- **Governs:** one sentence.
- **Depends on:** other modules it assumes, if any. A project that replaces a module must
  check what depends on it.
- **Settings:** the `config/` keys it reads.

## Where a project's choices live

In the project's settings inside the Trireme instance, versioned like everything else
there: who changed what, when and why. The fleet records the exact doctrine and module
versions it is running.
