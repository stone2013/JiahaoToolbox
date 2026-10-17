# Actual output verification
- MP4: SMV6_Intro_v3_1080p60.mp4
- ffprobe measured H.264, yuv420p, 1920x1080, r_frame_rate / avg_frame_rate 60/1, 180 decoded frames, video/container 3.000 seconds.
- Full FFmpeg audio/video decode: exit0, no decoding errors.
- TypeScript tsc --noEmit: passed.
- Actual encoded frames 0,20,45,70,100,150,179 extracted and visually inspected. Black opening, white wordmark, center-expanding blue line, fading tagline and settled hold confirmed. No source video/image scaling: all visual content authored as native DOM text/geometry at scale1.
- Audio: original 48kHz stereo synthesis, AAC output, measured peak -21.1dBFS, mean -31.5dBFS. No clipping; fade to silence by2.9s. Subjective speaker/headphone listening not performed in this automation environment.
- Fonts: installed Arial on this Windows machine. Fonts are not redistributed; different system fonts may change a future render.
- No existing SMV6 project or source footage overwritten.
