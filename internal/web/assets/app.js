// The page of pulse: the buttons call the API, a card leaves the paper only
// after the service said that its mark is on disk, and a click on a
// repository opens it in the reader. Opening the reader marks nothing.
(function () {
  'use strict';

  var showsHidden = document.body.dataset.view === 'hidden';
  // base is the path of the day of the page, day its date when it is a past
  // day: the marks of the page are made on that day.
  var base = document.body.dataset.base || '/';
  var day = document.body.dataset.day || '';
  // latest says that the page is today's: it shows the latest snapshots.
  var latest = document.body.dataset.latest === 'true';
  var shown = document.getElementById('shown');
  var hidden = document.getElementById('hidden');
  var empty = document.getElementById('empty');
  var notice = document.getElementById('notice');
  var seenAll = document.querySelector('[data-action="seen-all"]');
  var refresh = document.querySelector('[data-action="refresh"]');
  var reader = document.getElementById('reader');
  var sheet = reader.querySelector('.reader-sheet');
  var readerBody = document.getElementById('reader-body');
  var readerPlace = document.getElementById('reader-place');
  // reading is the key of the repository in the reader.
  var reading = null;
  var asked = 0;

  // post sends JSON to the API and resolves with its answer, or rejects with
  // the reason the service gave.
  function post(path, body) {
    if (day) {
      body.day = day;
    }
    if (latest) {
      body.latest = true;
    }
    return fetch(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    }).then(function (response) {
      return response.json().catch(function () { return {}; }).then(function (answer) {
        if (!response.ok) {
          throw new Error(answer.error || 'pulse answered ' + response.status);
        }
        return answer;
      });
    }, function () {
      throw new Error('pulse does not answer: is the service running?');
    });
  }

  function list(parent) {
    return parent ? Array.prototype.slice.call(parent.querySelectorAll('.card[data-repo]')) : [];
  }

  function shownCards() {
    return list(shown);
  }

  // cards are the repositories the reader walks through, in the order of
  // the page.
  function cards() {
    return shownCards().concat(list(hidden));
  }

  function cardOf(key) {
    return cards().filter(function (card) { return card.dataset.repo === key; })[0] || null;
  }

  // update shows the counts of the answer. When the last card left, the
  // service says what to show in its place; when it has more to show than
  // this page knows of, the page is loaded again.
  function update(answer) {
    if (answer.reload) {
      window.location.reload();
      return;
    }
    document.querySelector('[data-count="new"]').textContent = answer.new;
    document.querySelector('[data-count="hidden"]').textContent = answer.hidden;
    var left = shownCards().length;
    if (left === 0 && !answer.empty) {
      window.location.reload();
      return;
    }
    empty.innerHTML = left === 0 ? answer.empty : '';
    seenAll.disabled = left === 0;
    document.title = (left > 0 ? '(' + left + ') ' : '') + 'pulse · trend-watch';
  }

  function say(text) {
    notice.textContent = text;
    notice.hidden = !text;
  }

  function fail(holder, text) {
    var line = holder.querySelector('.card-error');
    line.textContent = text;
    line.hidden = !text;
  }

  function busy(holder, on) {
    holder.classList.toggle('is-busy', on);
    Array.prototype.forEach.call(holder.querySelectorAll('button, input'), function (control) {
      control.disabled = on;
    });
  }

  // mark sends "Interesting" or "Skip" from a card or from the reader. The
  // card leaves the paper; in the hidden view it moves to the hidden ones,
  // as the service renders it now. The reader moves to the next repository.
  function mark(holder, action) {
    var key = holder.dataset.repo;
    var inReader = readerBody.contains(holder);
    var comment = holder.querySelector('[name="comment"]').value;
    fail(holder, '');
    busy(holder, true);
    post('/api/marks', { action: action, repo: holder.dataset.name, comment: comment }).then(function (answer) {
      var card = cardOf(key);
      var after = inReader ? neighbour(key, 1) : null;
      var next = null;
      if (showsHidden && hidden && answer.card) {
        var template = document.createElement('template');
        template.innerHTML = answer.card.trim();
        next = template.content.firstElementChild;
      }
      if (card && next && card.parentNode === hidden) {
        hidden.replaceChild(next, card);
      } else {
        if (card) {
          card.remove();
        }
        if (next) {
          hidden.insertBefore(next, hidden.firstChild);
        }
      }
      update(answer);
      if (inReader) {
        if (after && after.dataset.repo !== key && shown.contains(after)) {
          open(after.dataset.repo);
        } else {
          close();
        }
      }
    }, function (error) {
      busy(holder, false);
      fail(holder, error.message);
    });
  }

  // neighbour returns the card a step away from the one of key.
  function neighbour(key, step) {
    var all = cards();
    for (var i = 0; i < all.length; i++) {
      if (all[i].dataset.repo === key) {
        return all[i + step] || null;
      }
    }
    return null;
  }

  function place() {
    var all = cards();
    var at = -1;
    all.forEach(function (card, i) {
      if (card.dataset.repo === reading) {
        at = i;
      }
    });
    readerPlace.textContent = at < 0 ? '' : (at + 1) + ' of ' + all.length;
    reader.querySelector('[data-action="prev-story"]').disabled = at <= 0;
    reader.querySelector('[data-action="next-story"]').disabled = at < 0 || at >= all.length - 1;
  }

  // open shows a repository in the reader. It asks the service for the
  // story and changes nothing.
  function open(key) {
    var card = cardOf(key);
    var name = card ? card.dataset.name : key;
    var ask = ++asked;
    reading = key;
    reader.hidden = false;
    document.body.classList.add('is-reading');
    readerBody.classList.add('is-loading');
    if (!readerBody.firstElementChild) {
      readerBody.innerHTML = '<p class="reader-wait">Opening…</p>';
    }
    place();
    if (window.location.hash.slice(1) !== name) {
      history.replaceState(null, '', '#' + name);
    }
    sheet.focus({ preventScroll: true });
    fetch(base + 'repos/' + name.split('/').map(encodeURIComponent).join('/')).then(function (response) {
      return response.text().then(function (text) {
        if (!response.ok) {
          throw new Error(text.trim() || 'pulse answered ' + response.status);
        }
        return text;
      });
    }, function () {
      throw new Error('pulse does not answer: is the service running?');
    }).then(function (story) {
      if (ask !== asked) {
        return;
      }
      readerBody.innerHTML = story;
      readerBody.classList.remove('is-loading');
      readerBody.scrollTop = 0;
    }, function (error) {
      if (ask !== asked) {
        return;
      }
      readerBody.textContent = '';
      var line = document.createElement('p');
      line.className = 'reader-wait';
      line.textContent = error.message;
      readerBody.appendChild(line);
      readerBody.classList.remove('is-loading');
    });
  }

  function close() {
    asked++;
    reading = null;
    reader.hidden = true;
    readerBody.textContent = '';
    document.body.classList.remove('is-reading');
    history.replaceState(null, '', window.location.pathname);
  }

  function step(by) {
    var next = reading && neighbour(reading, by);
    if (next) {
      open(next.dataset.repo);
    }
  }

  document.addEventListener('submit', function (event) {
    var holder = event.target.closest('[data-repo]');
    if (!holder) {
      return;
    }
    event.preventDefault();
    mark(holder, 'interesting');
  });

  document.addEventListener('click', function (event) {
    var control = event.target.closest('[data-action]');
    if (!control || control.disabled) {
      return;
    }
    switch (control.dataset.action) {
      case 'read':
        // A click with a modifier is the browser's: it opens GitHub.
        if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) {
          return;
        }
        if (window.getSelection && String(window.getSelection())) {
          return;
        }
        event.preventDefault();
        open(control.closest('[data-repo]').dataset.repo);
        break;

      case 'close-reader':
        close();
        break;

      case 'prev-story':
        step(-1);
        break;

      case 'next-story':
        step(1);
        break;

      case 'skip':
        mark(control.closest('[data-repo]'), 'skip');
        break;

      case 'seen-all':
        // Exactly the cards this page shows now, not what a later load
        // would add.
        var marked = shownCards();
        say('');
        control.disabled = true;
        post('/api/seen', { repos: marked.map(function (card) { return card.dataset.name; }) }).then(function (answer) {
          if (showsHidden) {
            window.location.reload();
            return;
          }
          marked.forEach(function (card) { card.remove(); });
          update(answer);
        }, function (error) {
          control.disabled = false;
          say(error.message);
        });
        break;

      case 'refresh':
        say('');
        control.disabled = true;
        control.textContent = 'Refreshing…';
        post('/api/refresh', {}).then(function () {
          window.location.reload();
        }, function (error) {
          control.disabled = false;
          control.textContent = 'Refresh';
          say(error.message);
        });
        break;
    }
  });

  document.addEventListener('keydown', function (event) {
    if (reader.hidden || event.metaKey || event.ctrlKey || event.altKey) {
      return;
    }
    if (event.key === 'Escape') {
      close();
      return;
    }
    if (event.target.closest('input, textarea')) {
      return;
    }
    if (event.key === 'j' || event.key === 'ArrowRight') {
      step(1);
    } else if (event.key === 'k' || event.key === 'ArrowLeft') {
      step(-1);
    }
  });

  if (refresh) {
    refresh.disabled = false;
  }
  // The address remembers the open repository across a reload.
  var named = decodeURIComponent(window.location.hash.slice(1)).toLowerCase();
  if (named && cardOf(named)) {
    open(named);
  }
})();
