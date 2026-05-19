# televid
A simple app to generate video telemetry overlays from TCX files. 

Check out an example video here: https://www.youtube.com/watch?v=6M2JsSgMb5o

I wrote this so I could stop using Dashware but still have overlay files. It's extremely specialized for my own usage but feel free to take and modify. I'll review PRs but the main purpose of this repo is for my own use-cases so if it's a feature / change that would hinder that then it likely won't be merged.

## How to use

1. **Download your TCX file**: The easiest way is to add `/export_tcx` to the end of a Strava activity URL (e.g., `https://www.strava.com/activities/18171754244/export_tcx`).
2. **Run the script**: Execute `run.ps1` and pass the filename of your TCX file as the first parameter.
   - **Prerequisite**: Ensure [FFMPEG](https://ffmpeg.org/) is installed and available in your PATH.
   - **Process**: The application generates 1920x1080 still images from your telemetry, and FFMPEG converts them into a `.mov` file using the `prores` codec.
   - **Note**: The `prores` codec supports transparency, so you don't need to use green screen settings.
3. **Import to video editor**: Load `output.mov` into your preferred video editor. You can then delete the generated image files to save space.

Tada!