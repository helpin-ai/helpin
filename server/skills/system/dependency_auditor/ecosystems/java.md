# Java / JVM Dependency Audits

Java covers Maven, Gradle, and common JVM dependency declarations for Java, Kotlin, Scala, Spring, Android JVM modules, and related build files. Treat JavaScript packages inside the same repository as Node dependencies, not Java dependencies.

## Parse
- Discover Maven files: `pom.xml`, parent POMs, module POMs, `dependencyManagement`, profiles, properties, BOM imports, and plugin dependencies when they are explicitly declared for build/runtime behavior.
- Discover Gradle files: `build.gradle`, `build.gradle.kts`, `settings.gradle`, `settings.gradle.kts`, version catalogs (`gradle/libs.versions.toml`), convention plugins, `buildSrc`, and included builds.
- Direct dependencies include explicit Maven `<dependency>` entries and Gradle dependency declarations such as `implementation`, `api`, `compileOnly`, `runtimeOnly`, `testImplementation`, annotation processors, platform/BOM declarations, and version-catalog aliases.
- Resolve property, BOM, parent, platform, and version-catalog indirection where possible. If the version cannot be resolved from repository files, mark the dependency unresolvable.
- Use lockfiles and reports such as Gradle dependency locks or Maven effective POM output only as supporting context. Do not create tasks for transitive-only dependencies unless `include_indirect` is true.

## Verify Versions
- Preferred public registry source: Maven Central Search API using `https://search.maven.org/solrsearch/select?q=g:<groupId>+AND+a:<artifactId>&rows=20&wt=json`.
- For all versions of a coordinate, use Maven Central GAV search with `core=gav`, or Maven metadata at `https://repo1.maven.org/maven2/<group path>/<artifactId>/maven-metadata.xml`.
- Batch-friendly CLI options may be used when project tooling is available and safe: Maven effective POM/dependency plugin goals and Gradle dependency/version catalog reports. Do not run tasks that mutate the project.
- Exclude snapshots, milestones, release candidates, alphas, betas, and timestamped snapshots unless the current declaration is already prerelease/snapshot or explicitly allows them.
- Respect repository declarations. For private repositories or custom artifact repositories, verify only when the metadata is accessible; otherwise mark unresolvable.

## Classify And Describe
- Dependency identity is `groupId:artifactId` plus repository identity.
- Maven/Gradle versions are not always semver. Classify conservatively: first numeric segment change is `major`, second is `minor`, later segment is `patch`, and incomparable schemes are `unknown`.
- Treat BOM/platform updates separately from ordinary artifacts. If a BOM controls many dependency versions, create one task for the BOM, not one task per transitive managed artifact.
- Include affected build files, dependency scope/configuration, property/catalog key, BOM/platform source, current declaration, resolved version if found, latest verified version, Java/Kotlin/Gradle/Maven compatibility notes, repository/source, and advisory/changelog URLs in each task.
