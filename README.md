# televid
A simple app to generate video telemetry overlays from TCX files. 

Check out an example video here: https://www.youtube.com/watch?v=6M2JsSgMb5o

I wrote this so I could stop using Dashware but still have overlay files. It's extremely specialized for my own usage but feel free to take and modify. I'll review PRs but the main purpose of this repo is for my own use-cases so if it's a feature / change that would hinder that then it likely won't be merged.

## How to use
1. Download your tcx file from your activity, the easiest way I know is to add "/export_tcx" to the end of a strava activity URL e.g. https://www.strava.com/activities/18171754244/export_tcx
2. Replace the filename in main.go
3. Run the "run.ps1" script (or just read the script and run each command separately). Note that you'll need to have FFMPEG installed and available on your path. The application will generate a bunch of still images from your telemetry at 1920x1080 and then FFMPEG will take those and create a .mov file using the "prores" codec. This codec lets me export the video with transparency so I don't have to fiddle with green screen settings like I did with Dashware.
4. Take the "output.mov" file and load it into your video editor of choice. At this point you can delete the leftover img files to reclaim some disk space.

Tada!