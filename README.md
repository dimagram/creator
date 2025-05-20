dimagram creator
----------------

a tool to hand curate a feed of daily pictures.

the "official" feed is at dimagram.com for now. idk where i want to take this yet.
maybe self host this thing and make your own feeds?

    docker run -v /somewhere/dimagram/data:/app/data -p 8080:8080 ghcr.io/dimagram/creator

and then have a daily cronjob that does 

    docker run \
        -e SFTP_HOST=storage.bunnycdn.com \
        -e SFTP_PORT=22 \
        -e SFTP_USER=your-sftp-username \
        -e SFTP_PASSWORD=blaetcsecure \
        -e BUNNY_API_KEY=your-api-key-here \
        -e BUNNY_CDN_URL=https://your-pullzone.b-cdn.net \
        -v /somewhere/dimagram/data:/app/data \
        ghcr.io/dimagram/creator publish
