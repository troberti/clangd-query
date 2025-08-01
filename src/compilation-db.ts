import { execa } from 'execa';
import * as path from 'path';
import * as fs from 'fs/promises';

/**
 * Ensures a compile_commands.json exists in the .cache/clangd-query directory.
 *
 * This function follows the startup logic from the technical specification:
 * 1. First checks if compile_commands.json already exists in .cache/clangd-query/build
 * 2. If not, checks if CMakeLists.txt exists in the project root
 * 3. If CMakeLists.txt exists, runs CMake to generate compile_commands.json
 * 4. All generated files are kept within .cache/clangd-query to avoid interfering with
 *    the user's build setup or other tools like VSCode's clangd
 *
 * The compile_commands.json file is essential for clangd to understand how to
 * compile each source file in the project, enabling accurate code analysis,
 * navigation, and symbol indexing.
 *
 * @param projectRoot - The absolute path to the C++ project root directory
 * @returns The path to the directory containing compile_commands.json
 * @throws Error if no CMakeLists.txt is found or if CMake fails to generate
 *         the compilation database
 */
export async function ensureCompileCommands(projectRoot: string): Promise<string> {
    // Our dedicated directory for clangd-query within the .cache directory
    const cacheDir = path.join(projectRoot, '.cache');
    const queryDir = path.join(cacheDir, 'clangd-query');
    const queryBuildDir = path.join(queryDir, 'build');
    const queryCompileCommandsPath = path.join(queryBuildDir, 'compile_commands.json');

    // Check if we already have compile_commands.json in our clangd-query directory
    try {
        await fs.access(queryCompileCommandsPath);
        console.log('Found existing compile_commands.json in .cache/clangd-query/build.');
        return queryBuildDir; // Return the directory containing compile_commands.json
    } catch {
        // It doesn't exist in our directory, we need to generate it
        console.log('compile_commands.json not found in .cache/clangd-query/build.');
    }

    // Check if CMakeLists.txt exists in project root
    const cmakeListsPath = path.join(projectRoot, 'CMakeLists.txt');
    try {
        await fs.access(cmakeListsPath);
    } catch {
        throw new Error(
            `Error: compile_commands.json not found in .cache/clangd-query/build and no CMakeLists.txt is present to generate it. Please configure your C++ project first.`
        );
    }

    console.log('compile_commands.json not found. Attempting to generate from CMakeLists.txt...');

    try {
        // Ensure the build directory exists
        await fs.mkdir(queryBuildDir, { recursive: true });

        // Run CMake to generate compile_commands.json in our directory
        console.log(`Running cmake in ${queryBuildDir}...`);
        const result = await execa('cmake', [
            '-S', projectRoot,     // Source directory
            '-B', queryBuildDir,     // Build directory (our .clangd-query/build)
            '-DCMAKE_EXPORT_COMPILE_COMMANDS=ON'
        ], {
            cwd: projectRoot,
            stderr: 'pipe',
            stdout: 'pipe'
        });

        // Verify the file was created
        await fs.access(queryCompileCommandsPath);

        console.log('Successfully generated compile_commands.json.');
        return queryBuildDir; // Return the directory containing compile_commands.json

    } catch (error: any) {
        // Provide a helpful error message with the actual output from CMake
        throw new Error(
            `CMake failed to generate compile_commands.json. Please check your CMakeLists.txt and ensure 'cmake' is installed.\n---\nCMake Output:\n${error.stderr || error.message}`
        );
    }
}