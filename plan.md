As a developer who have certain experience on software development (backend role). 
I want you to create a webapp that will be running in local machine and can be run via docker as in the first phase. 

The app will be using as a tool for watching anime. It will support the following things in the first phase
- Saving anime watching process.
- Loading custom subtitles.
- No need for account setting right now but the development must setup in a way for having profile support (will be doing in the next phase)
- <IMPORTANT> must fully support migaku for immersion learning like nextflix/youtube
- <IMPORTANT> must have a way to get anime from free api platform and the implementation must impplement this in a way so that we can easily add, remove broken API Of course this heavily depends on the design of the external API itself, but we might implement it in a clear way (dependencies injection, factory design pattern).
- Support for Macos (but in next phase we are going to support for Windows).
The user of this app can mine content from this with fully support from migaku. But i'm not sure if this depend on migaku side or not. Since animelon (dead now) did it, you might investigate and give me answer first before proceeding. Potentially absplayer but proritize migaku before absplayer.

For the anime source. At this phase, you must have a default set of source reference from what ani-cli used (use github mcp to research and fine out, i think it's allanime api).

- The frontend techstack is all on you. Use the most suitatble one. I prefer super minimal UI. Not fancy gradient stuffs, basic html css or simple react is fine. Make it like 2010 2015 website. I prefer UX over UI. Note: [Manatan](https://github.com/KolbyML/Manatan) don't even use html or css, try things that are the most suitable
- The backend i prefer go with gin. For the db use Gin. I prefer simple code structure but no hardcode element. Something following [register-system-be](https://github.com/NoNameHCMUT/register-system-be) is fine but you might want to tune it for more "correct version" as the code structure here is made up by me. 
- The development will resolve around a dev container and we will run/test/debug inside that container. For the final product we will have a production image that when user install it. They will run that image instead. For the dev container, we will have minimum these following scripts, create_docker, run_docker, install (if we have install step), run. The install and run script will be ran inside docker. For the client, if you know a better way of developing, please tell me. 

The development process here is all based on my experience. You can reference to other open source app that share the same structure and reference their development process, give me some proposal. 

The env config instead of heavily depends on the docker env. We should have a .config/.yaml file for it so we don't have to spin the docker everytime we run.

DO NOT CODE FIRST, you will be giving me an infrastructure looking, docs, details implementation plan each a file. 
