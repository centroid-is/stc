# Phase 29 deferred items

## From 29-02

- **Variable-level StructuredType on an ARRAY OF struct is published as an Object.**
  The legacy project (skammtalinur-legacy) declares
  `{attribute 'OPC.UA.DA.StructuredType' := '1'} recipes : ARRAY [1..3] OF ST_LineRecipe;`
  and the real hmi/keymappings.json reads `ns=4;s=GVL_BatchLines.recipes` (key LineRecipes)
  as one value. pkg/opcua Build treats the array as a container, so the Value read
  returns BadAttributeIdInvalid. TF6100 most likely serves a Variable of
  ExtensionObject[] here. Out of scope for 29-02 (pkg/opcua Build/convert change plus
  a golden update); confirm the TF6100 shape with `stc opcua snapshot` first.
  Found by TestHMIKeymappingsReal with STC_HMI_PROJECT=skammtalinur-legacy/sildarvinnsla.tsproj:
  319 good, 1 bad (this id), 108 skipped.
